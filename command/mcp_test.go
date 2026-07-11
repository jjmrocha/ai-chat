package command

import (
	"context"
	"errors"
	"testing"

	"github.com/jjmrocha/ai-toolkit/mcp"
	"github.com/stretchr/testify/assert"
)

func TestMCPCommand(t *testing.T) {
	t.Run("name", func(t *testing.T) {
		assert.Equal(t, "mcp", MCP(&mockedMCPController{}).Name())
	})

	t.Run("help", func(t *testing.T) {
		assert.NotEmpty(t, MCP(&mockedMCPController{}).Help())
	})

	t.Run("no args with no servers", func(t *testing.T) {
		// given
		ctx := &mockedContext{}
		cmd := MCP(&mockedMCPController{
			getMCPsFunc: func() []mcp.Status { return nil },
		})

		// when
		cmd.Run(ctx, "")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, "No MCP servers registered.", ctx.printed[0].text)
		}
	})

	t.Run("no args lists servers", func(t *testing.T) {
		// given
		ctx := &mockedContext{}
		cmd := MCP(&mockedMCPController{
			getMCPsFunc: func() []mcp.Status {
				return []mcp.Status{
					{Name: "server-a", Active: true},
					{Name: "server-b", Active: false},
				}
			},
		})

		// when
		cmd.Run(ctx, "")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, Info, ctx.printed[0].kind)
		}
	})

	t.Run("start server", func(t *testing.T) {
		// given
		var started string
		ctx := &mockedContext{}
		cmd := MCP(&mockedMCPController{
			startFunc: func(_ context.Context, name string) error {
				started = name
				return nil
			},
		})

		// when
		cmd.Run(ctx, "on my-server")

		// then
		assert.Equal(t, "my-server", started)
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, "MCP my-server started.", ctx.printed[0].text)
		}
	})

	t.Run("stop server", func(t *testing.T) {
		// given
		var stopped string
		ctx := &mockedContext{}
		cmd := MCP(&mockedMCPController{
			stopFunc: func(name string) error {
				stopped = name
				return nil
			},
		})

		// when
		cmd.Run(ctx, "off my-server")

		// then
		assert.Equal(t, "my-server", stopped)
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, "MCP my-server stopped.", ctx.printed[0].text)
		}
	})

	t.Run("start with no name and single server resolves", func(t *testing.T) {
		// given
		var started string
		ctx := &mockedContext{}
		cmd := MCP(&mockedMCPController{
			getMCPsFunc: func() []mcp.Status {
				return []mcp.Status{{Name: "sole-server", Active: false}}
			},
			startFunc: func(_ context.Context, name string) error {
				started = name
				return nil
			},
		})

		// when
		cmd.Run(ctx, "on")

		// then
		assert.Equal(t, "sole-server", started)
	})

	t.Run("start with no name and no servers prints error", func(t *testing.T) {
		// given
		ctx := &mockedContext{}
		cmd := MCP(&mockedMCPController{
			getMCPsFunc: func() []mcp.Status { return nil },
		})

		// when
		cmd.Run(ctx, "on")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, Error, ctx.printed[0].kind)
		}
	})

	t.Run("start error prints error", func(t *testing.T) {
		// given
		ctx := &mockedContext{}
		cmd := MCP(&mockedMCPController{
			startFunc: func(_ context.Context, name string) error {
				return errors.New("connection failed")
			},
		})

		// when
		cmd.Run(ctx, "on my-server")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, Error, ctx.printed[0].kind)
			assert.Equal(t, "Error: connection failed", ctx.printed[0].text)
		}
	})

	t.Run("invalid action prints usage", func(t *testing.T) {
		// given
		ctx := &mockedContext{}
		cmd := MCP(&mockedMCPController{})

		// when
		cmd.Run(ctx, "restart my-server")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, Error, ctx.printed[0].kind)
			assert.Equal(t, "Usage: /mcp [on|off] [name]", ctx.printed[0].text)
		}
	})
}
