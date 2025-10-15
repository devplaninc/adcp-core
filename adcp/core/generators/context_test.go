package generators

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	core2 "github.com/devplaninc/adcp-core/adcp/core"
	"github.com/devplaninc/adcp-core/adcp/core/utils"
	"github.com/devplaninc/adcp/clients/go/adcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string {
	return &s
}

func contextEntry(path string, from *adcp.ContextFrom) *adcp.ContextEntry {
	return adcp.ContextEntry_builder{Path: path, From: from}.Build()
}

func textFrom(text string) *adcp.ContextFrom {
	return adcp.ContextFrom_builder{Text: strPtr(text)}.Build()
}

func cmdFrom(cmd string) *adcp.ContextFrom {
	return adcp.ContextFrom_builder{Cmd: strPtr(cmd)}.Build()
}

func githubFrom(path string) *adcp.ContextFrom {
	return adcp.ContextFrom_builder{
		Github: adcp.GitReference_builder{Path: path}.Build(),
	}.Build()
}

func combinedFrom(items ...*adcp.CombinedContextSource_Item) *adcp.ContextFrom {
	return adcp.ContextFrom_builder{
		Combined: adcp.CombinedContextSource_builder{Items: items}.Build(),
	}.Build()
}

func combinedTextItem(text string) *adcp.CombinedContextSource_Item {
	return adcp.CombinedContextSource_Item_builder{Text: strPtr(text)}.Build()
}

func combinedCmdItem(cmd string) *adcp.CombinedContextSource_Item {
	return adcp.CombinedContextSource_Item_builder{Cmd: strPtr(cmd)}.Build()
}

func combinedGithubItem(path string) *adcp.CombinedContextSource_Item {
	return adcp.CombinedContextSource_Item_builder{
		Github: adcp.GitReference_builder{Path: path}.Build(),
	}.Build()
}

func TestContext_Materialize(t *testing.T) {
	tests := []struct {
		name     string
		context  *adcp.Context
		genCtx   *core2.GenerationContext
		wantErr  string
		validate func(*testing.T, *adcp.MaterializedResult)
	}{
		{
			name:    "nil context",
			wantErr: "context cannot be nil",
		},
		{
			name:    "nil entries",
			context: adcp.Context_builder{}.Build(),
			validate: func(t *testing.T, result *adcp.MaterializedResult) {
				assert.NotNil(t, result)
				assert.Empty(t, result.GetEntries())
			},
		},
		{
			name:    "empty entries",
			context: adcp.Context_builder{Entries: []*adcp.ContextEntry{}}.Build(),
			validate: func(t *testing.T, result *adcp.MaterializedResult) {
				assert.NotNil(t, result)
				assert.Empty(t, result.GetEntries())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Context{}
			result, err := c.Materialize(context.Background(), tt.context, tt.genCtx)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestContext_MaterializeEntry(t *testing.T) {
	tests := []struct {
		name    string
		entry   *adcp.ContextEntry
		genCtx  *core2.GenerationContext
		wantErr string
	}{
		{
			name:    "empty path",
			entry:   adcp.ContextEntry_builder{}.Build(),
			wantErr: "entry path cannot be empty",
		},
		{
			name:    "no from source",
			entry:   adcp.ContextEntry_builder{Path: "test.txt"}.Build(),
			wantErr: "entry must have a 'from' source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Context{}
			_, err := c.materializeEntry(context.Background(), tt.entry, tt.genCtx)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestContext_FetchContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("github content"))
	}))
	defer server.Close()

	tests := []struct {
		name    string
		from    *adcp.ContextFrom
		genCtx  *core2.GenerationContext
		want    string
		wantErr string
	}{
		{
			name: "text source",
			from: textFrom("hello world"),
			want: "hello world",
		},
		{
			name: "command source",
			from: cmdFrom("echo 'test output'"),
			want: "test output\n",
		},
		{
			name: "github source",
			from: githubFrom(server.URL),
			want: "github content",
		},
		{
			name: "combined source",
			from: combinedFrom(
				combinedTextItem("# Overview: "),
				combinedCmdItem("echo 'from command'"),
				combinedTextItem("\n# End"),
			),
			want: "# Overview: from command\n\n# End",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Context{}
			content, err := c.fetchContent(context.Background(), tt.from, tt.genCtx)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, content)
		})
	}
}

func TestUtils_ExecuteCommand(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		want    string
		wantErr bool
	}{
		{
			name: "success",
			cmd:  "echo 'test output'",
			want: "test output\n",
		},
		{
			name:    "empty command",
			cmd:     "",
			wantErr: true,
		},
		{
			name:    "failed command",
			cmd:     "exit 1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := utils.ExecuteCommand(context.Background(), tt.cmd)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, content)
		})
	}
}

func TestUtils_FetchGithub(t *testing.T) {
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("file content from github"))
	}))
	defer successServer.Close()

	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer errorServer.Close()

	tests := []struct {
		name    string
		ref     *adcp.GitReference
		want    string
		wantErr bool
	}{
		{
			name: "success",
			ref:  adcp.GitReference_builder{Path: successServer.URL}.Build(),
			want: "file content from github",
		},
		{
			name:    "nil reference",
			ref:     nil,
			wantErr: true,
		},
		{
			name:    "empty path",
			ref:     adcp.GitReference_builder{}.Build(),
			wantErr: true,
		},
		{
			name:    "HTTP error",
			ref:     adcp.GitReference_builder{Path: errorServer.URL + "/notfound"}.Build(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := utils.FetchGithub(context.Background(), tt.ref)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, content)
		})
	}
}

func TestContext_FetchCombined(t *testing.T) {
	tests := []struct {
		name     string
		combined *adcp.CombinedContextSource
		genCtx   *core2.GenerationContext
		want     string
		wantErr  string
	}{
		{
			name: "success",
			combined: adcp.CombinedContextSource_builder{
				Items: []*adcp.CombinedContextSource_Item{
					combinedTextItem("# Overview: "),
					combinedCmdItem("echo 'from command'"),
					combinedTextItem("\n# End"),
				},
			}.Build(),
			want: "# Overview: from command\n\n# End",
		},
		{
			name:    "nil combined",
			wantErr: "combined source cannot be nil",
		},
		{
			name:     "empty items",
			combined: adcp.CombinedContextSource_builder{}.Build(),
			want:     "",
		},
		{
			name: "failed item",
			combined: adcp.CombinedContextSource_builder{
				Items: []*adcp.CombinedContextSource_Item{
					combinedTextItem("text1"),
					combinedCmdItem("exit 1"),
				},
			}.Build(),
			wantErr: "failed to fetch combined item",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Context{}
			content, err := c.fetchCombined(context.Background(), tt.combined, tt.genCtx)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, content)
		})
	}
}

func TestContext_FetchCombinedItem(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("github content"))
	}))
	defer server.Close()

	tests := []struct {
		name    string
		item    *adcp.CombinedContextSource_Item
		genCtx  *core2.GenerationContext
		want    string
		wantErr string
	}{
		{
			name: "text",
			item: combinedTextItem("test text"),
			want: "test text",
		},
		{
			name: "cmd",
			item: combinedCmdItem("echo 'cmd output'"),
			want: "cmd output\n",
		},
		{
			name: "github",
			item: combinedGithubItem(server.URL),
			want: "github content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Context{}
			content, err := c.fetchCombinedItem(context.Background(), tt.item, tt.genCtx)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, content)
		})
	}
}

func ExampleContext_Materialize() {
	c := &Context{}

	ctx := adcp.Context_builder{
		Entries: []*adcp.ContextEntry{
			contextEntry("example.txt", textFrom("This is example content")),
		},
	}.Build()

	result, err := c.Materialize(context.Background(), ctx, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	for _, entry := range result.GetEntries() {
		if entry.HasFile() {
			file := entry.GetFile()
			fmt.Printf("Path: %s, Content Length: %d\n", file.GetPath(), len(file.GetContent()))
		}
	}
	// Output: Path: example.txt, Content Length: 23
}
