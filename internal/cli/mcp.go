package cli

import (
	"github.com/dungeonbooks/tools/internal/enrich"
	"github.com/dungeonbooks/tools/internal/mcpsrv"
	"github.com/dungeonbooks/tools/internal/platform/config"
	"github.com/spf13/cobra"
)

func mcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Serve book lookup as MCP tools over stdio",
		Long: "Serve book lookup as MCP tools over stdio.\n\n" +
			"Exposes resolve_book and resolve_isbn to an MCP client, which speaks\n" +
			"JSON-RPC on this process's stdin/stdout — so nothing else may write to\n" +
			"stdout. Not meant to be run by hand; a client launches it. This repo\n" +
			"registers it for Claude Code in .mcp.json.",
		Args:         usageArgs(cobra.NoArgs),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.Load()
			svc := enrich.New(cfg.HardcoverToken, cfg.GoogleBooksKey)
			return mcpsrv.Run(cmd.Context(), svc)
		},
	}
}
