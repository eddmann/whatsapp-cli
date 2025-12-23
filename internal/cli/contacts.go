package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

var contactsQuery string

var contactsCmd = &cobra.Command{
	Use:   "contacts",
	Short: "List contacts",
	Long:  `List all contacts from WhatsApp.`,
	RunE:  runContacts,
}

func init() {
	rootCmd.AddCommand(contactsCmd)
	contactsCmd.Flags().StringVar(&contactsQuery, "query", "", "Filter by name")
}

func runContacts(cmd *cobra.Command, args []string) error {
	return WithConnection(func(db *store.DB, client *whatsapp.Client) error {
		contacts, err := client.WA.Store.Contacts.GetAllContacts(context.Background())
		if err != nil {
			return fmt.Errorf("failed to get contacts: %w", err)
		}

		var result []store.Contact
		queryLower := strings.ToLower(contactsQuery)

		for jid, contact := range contacts {
			name := contact.FullName
			if name == "" {
				name = contact.PushName
			}
			if name == "" {
				name = contact.BusinessName
			}

			if contactsQuery != "" {
				if !strings.Contains(strings.ToLower(name), queryLower) &&
					!strings.Contains(jid.User, queryLower) {
					continue
				}
			}

			c := store.Contact{
				JID:   jid.String(),
				Phone: jid.User,
			}
			if name != "" {
				c.Name = &name
			}
			result = append(result, c)
		}

		return Output(result)
	})
}
