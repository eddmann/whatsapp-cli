package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var aliasRemove bool

var aliasCmd = &cobra.Command{
	Use:   "alias [jid] [name]",
	Short: "Manage local aliases for JIDs",
	Long: `Manage local aliases for JIDs.

Without arguments, lists all aliases.
With JID and name, sets an alias.
With --remove, removes an alias.

Examples:
  whatsapp alias                                    # List all aliases
  whatsapp alias 1234567890@s.whatsapp.net "John"   # Set alias
  whatsapp alias 1234567890@s.whatsapp.net --remove # Remove alias`,
	RunE: runAlias,
}

func init() {
	rootCmd.AddCommand(aliasCmd)
	aliasCmd.Flags().BoolVar(&aliasRemove, "remove", false, "Remove alias")
}

func runAlias(cmd *cobra.Command, args []string) error {
	if err := EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// List all aliases
	if len(args) == 0 {
		aliases, err := ListAliases()
		if err != nil {
			return fmt.Errorf("failed to list aliases: %w", err)
		}
		return Output(aliases)
	}

	jid := args[0]

	// Remove alias
	if aliasRemove {
		if err := RemoveAlias(jid); err != nil {
			return fmt.Errorf("failed to remove alias: %w", err)
		}
		return OutputResult(map[string]any{
			"jid": jid,
		}, fmt.Sprintf("Removed alias for %s", jid))
	}

	// Set alias
	if len(args) < 2 {
		return fmt.Errorf("requires 2 args (jid and name)")
	}

	name := args[1]
	if err := SetAlias(jid, name); err != nil {
		return fmt.Errorf("failed to set alias: %w", err)
	}

	return OutputResult(map[string]any{
		"jid":  jid,
		"name": name,
	}, fmt.Sprintf("Set alias '%s' for %s", name, jid))
}
