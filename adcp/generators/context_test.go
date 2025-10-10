package generators

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devplaninc/adcp-core/adcp/utils"
	"github.com/devplaninc/adcp/clients/go/adcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to convert string to *string for builder pattern
func strPtr(s string) *string {
	return &s
}

func TestContext_Materialize_NilContext(t *testing.T) {
	c := &Context{}
	_, err := c.Materialize(context.Background(), nil)
	assert.Error(t, err, "expected error for nil context")
}

func TestContext_Materialize_NilEntries(t *testing.T) {
	c := &Context{}
	ctx := adcp.Context_builder{}.Build()
	result, err := c.Materialize(context.Background(), ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.GetEntries())
}

func TestContext_Materialize_EmptyEntries(t *testing.T) {
	c := &Context{}
	ctx := adcp.Context_builder{
		Entries: []*adcp.ContextEntry{},
	}.Build()
	result, err := c.Materialize(context.Background(), ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.GetEntries())
}

func TestContext_MaterializeEntry_EmptyPath(t *testing.T) {
	c := &Context{}
	entry := adcp.ContextEntry_builder{}.Build()
	_, err := c.materializeEntry(context.Background(), entry)
	assert.Error(t, err, "expected error for empty path")
}

func TestContext_MaterializeEntry_NoFromSource(t *testing.T) {
	c := &Context{}
	entry := adcp.ContextEntry_builder{
		Path: "test.txt",
	}.Build()
	_, err := c.materializeEntry(context.Background(), entry)
	assert.Error(t, err, "expected error for missing 'from' source")
}

func TestContext_FetchContent_TextSource(t *testing.T) {
	c := &Context{}
	from := adcp.ContextFrom_builder{
		Text: strPtr("hello world"),
	}.Build()

	content, err := c.fetchContent(context.Background(), from)
	require.NoError(t, err)
	assert.Equal(t, "hello world", content)
}

func TestContext_ExecuteCommand_Success(t *testing.T) {
	c := &Context{}
	_ = c
	content, err := utils.ExecuteCommand(context.Background(), "echo 'test output'")
	require.NoError(t, err)
	assert.Equal(t, "test output\n", content)
}

func TestContext_ExecuteCommand_EmptyCommand(t *testing.T) {
	c := &Context{}
	_ = c
	_, err := utils.ExecuteCommand(context.Background(), "")
	assert.Error(t, err, "expected error for empty command")
}

func TestContext_ExecuteCommand_FailedCommand(t *testing.T) {
	c := &Context{}
	_ = c
	_, err := utils.ExecuteCommand(context.Background(), "exit 1")
	assert.Error(t, err, "expected error for failed command")
}

func TestContext_FetchGithub_Success(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("file content from github"))
	}))
	defer server.Close()

	c := &Context{}
	_ = c
	ref := &adcp.GitReference{}
	// Use raw.githubusercontent.com format to bypass URL conversion
	ref.SetPath(server.URL)

	content, err := utils.FetchGithub(context.Background(), ref)
	require.NoError(t, err)
	assert.Equal(t, "file content from github", content)
}

func TestContext_FetchGithub_NilReference(t *testing.T) {
	c := &Context{}
	_ = c
	_, err := utils.FetchGithub(context.Background(), nil)
	assert.Error(t, err, "expected error for nil reference")
}

func TestContext_FetchGithub_EmptyPath(t *testing.T) {
	c := &Context{}
	_ = c
	ref := adcp.GitReference_builder{}.Build()
	_, err := utils.FetchGithub(context.Background(), ref)
	assert.Error(t, err, "expected error for empty path")
}

func TestContext_FetchGithub_HTTPError(t *testing.T) {
	// Create a mock HTTP server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := &Context{}
	_ = c
	ref := &adcp.GitReference{}
	ref.SetPath(server.URL + "/notfound")

	_, err := utils.FetchGithub(context.Background(), ref)
	assert.Error(t, err, "expected error for 404 status")
}

func TestContext_FetchCombined_Success(t *testing.T) {
	c := &Context{}

	combined := adcp.CombinedContextSource_builder{
		Items: []*adcp.CombinedContextSource_Item{
			adcp.CombinedContextSource_Item_builder{
				Text: strPtr("# Overview: "),
			}.Build(),
			adcp.CombinedContextSource_Item_builder{
				Cmd: strPtr("echo 'from command'"),
			}.Build(),
			adcp.CombinedContextSource_Item_builder{
				Text: strPtr("\n# End"),
			}.Build(),
		},
	}.Build()

	content, err := c.fetchCombined(context.Background(), combined)
	require.NoError(t, err)
	assert.Equal(t, "# Overview: from command\n\n# End", content)
}

func TestContext_FetchCombined_NilCombined(t *testing.T) {
	c := &Context{}
	_, err := c.fetchCombined(context.Background(), nil)
	assert.Error(t, err, "expected error for nil combined source")
}

func TestContext_FetchCombined_EmptyItems(t *testing.T) {
	c := &Context{}
	combined := adcp.CombinedContextSource_builder{}.Build()
	content, err := c.fetchCombined(context.Background(), combined)
	require.NoError(t, err)
	assert.Empty(t, content)
}

func TestContext_FetchCombined_FailedItem(t *testing.T) {
	c := &Context{}

	combined := adcp.CombinedContextSource_builder{
		Items: []*adcp.CombinedContextSource_Item{
			adcp.CombinedContextSource_Item_builder{
				Text: strPtr("text1"),
			}.Build(),
			adcp.CombinedContextSource_Item_builder{
				Cmd: strPtr("exit 1"), // This will fail
			}.Build(),
		},
	}.Build()

	_, err := c.fetchCombined(context.Background(), combined)
	assert.Error(t, err, "expected error for failed combined item")
}

func TestContext_FetchCombinedItem_Text(t *testing.T) {
	c := &Context{}
	item := adcp.CombinedContextSource_Item_builder{
		Text: strPtr("test text"),
	}.Build()

	content, err := c.fetchCombinedItem(context.Background(), item)
	require.NoError(t, err)
	assert.Equal(t, "test text", content)
}

func TestContext_FetchCombinedItem_Cmd(t *testing.T) {
	c := &Context{}
	item := adcp.CombinedContextSource_Item_builder{
		Cmd: strPtr("echo 'cmd output'"),
	}.Build()

	content, err := c.fetchCombinedItem(context.Background(), item)
	require.NoError(t, err)
	assert.Equal(t, "cmd output\n", content)
}

func TestContext_FetchCombinedItem_Github(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("github content"))
	}))
	defer server.Close()

	c := &Context{}
	item := adcp.CombinedContextSource_Item_builder{
		Github: adcp.GitReference_builder{
			Path: server.URL,
		}.Build(),
	}.Build()

	content, err := c.fetchCombinedItem(context.Background(), item)
	require.NoError(t, err)
	assert.Equal(t, "github content", content)
}

func ExampleContext_Materialize() {
	c := &Context{}

	ctx := adcp.Context_builder{
		Entries: []*adcp.ContextEntry{
			adcp.ContextEntry_builder{
				Path: "example.txt",
				From: adcp.ContextFrom_builder{
					Text: strPtr("This is example content"),
				}.Build(),
			}.Build(),
		},
	}.Build()

	result, err := c.Materialize(context.Background(), ctx)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Print materialized files
	for _, entry := range result.GetEntries() {
		if entry.HasFile() {
			file := entry.GetFile()
			fmt.Printf("Path: %s, Content Length: %d\n", file.GetPath(), len(file.GetContent()))
		}
	}
	// Output: Path: example.txt, Content Length: 23
}
