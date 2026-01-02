package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"go.mau.fi/whatsmeow/types"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

var groupsCmd = &cobra.Command{
	Use:   "groups [jid]",
	Short: "List groups or show group info",
	Long: `Without arguments, lists all groups.
With a JID, shows detailed group info including members.`,
	RunE: runGroups,
}

var groupsJoinCmd = &cobra.Command{
	Use:   "join <invite-code>",
	Short: "Join a group via invite code",
	Args:  cobra.ExactArgs(1),
	RunE:  runGroupsJoin,
}

var groupsLeaveCmd = &cobra.Command{
	Use:   "leave <jid>",
	Short: "Leave a group",
	Args:  cobra.ExactArgs(1),
	RunE:  runGroupsLeave,
}

var groupsRenameCmd = &cobra.Command{
	Use:   "rename <jid> <name>",
	Short: "Rename a group",
	Args:  cobra.ExactArgs(2),
	RunE:  runGroupsRename,
}

func init() {
	rootCmd.AddCommand(groupsCmd)
	groupsCmd.AddCommand(groupsJoinCmd)
	groupsCmd.AddCommand(groupsLeaveCmd)
	groupsCmd.AddCommand(groupsRenameCmd)
}

func runGroups(cmd *cobra.Command, args []string) error {
	return WithConnection(func(db *store.DB, client *whatsapp.Client) error {
		// If JID provided, show group info
		if len(args) > 0 {
			jid, err := types.ParseJID(args[0])
			if err != nil {
				return fmt.Errorf("invalid JID: %w", err)
			}

			info, err := client.WA.GetGroupInfo(context.Background(), jid)
			if err != nil {
				return fmt.Errorf("failed to get group info: %w", err)
			}

			var participants []store.Participant
			for _, p := range info.Participants {
				name := ""
				var lidStr, phoneStr *string

				lookupJID := p.JID
				if !p.PhoneNumber.IsEmpty() {
					lookupJID = p.PhoneNumber
				}

				if contact, err := client.WA.Store.Contacts.GetContact(context.Background(), lookupJID); err == nil {
					if contact.FullName != "" {
						name = contact.FullName
					} else if contact.PushName != "" {
						name = contact.PushName
					}
				}

				if name == "" && p.DisplayName != "" {
					name = p.DisplayName
				}

				if !p.LID.IsEmpty() {
					lid := p.LID.User
					lidStr = &lid

					phone := ""
					if !p.PhoneNumber.IsEmpty() {
						phone = p.PhoneNumber.User
						phoneStr = &phone
					}
					_ = db.StoreLIDMapping(lid, phone, name)
				}

				if !p.PhoneNumber.IsEmpty() && phoneStr == nil {
					phone := p.PhoneNumber.User
					phoneStr = &phone
				}

				participants = append(participants, store.Participant{
					JID:     p.JID.String(),
					LID:     lidStr,
					Phone:   phoneStr,
					IsAdmin: p.IsAdmin || p.IsSuperAdmin,
					Name:    name,
				})
			}

			return Output(store.GroupInfo{
				JID:          info.JID.String(),
				Name:         info.Name,
				Topic:        info.Topic,
				Created:      info.GroupCreated,
				CreatorJID:   info.OwnerJID.String(),
				Participants: participants,
			})
		}

		// List all groups from local database
		groups, err := db.ListChats(store.ListChatsOptions{OnlyGroups: true, Limit: 100})
		if err != nil {
			return fmt.Errorf("failed to list groups: %w", err)
		}

		return Output(groups)
	})
}

func runGroupsJoin(cmd *cobra.Command, args []string) error {
	inviteCode := args[0]

	return WithConnection(func(db *store.DB, client *whatsapp.Client) error {
		jid, err := client.WA.JoinGroupWithLink(context.Background(), inviteCode)
		if err != nil {
			return fmt.Errorf("failed to join group: %w", err)
		}

		return OutputResult(map[string]any{
			"jid": jid.String(),
		}, fmt.Sprintf("Joined group %s", jid.String()))
	})
}

func runGroupsLeave(cmd *cobra.Command, args []string) error {
	jid, err := types.ParseJID(args[0])
	if err != nil {
		return fmt.Errorf("invalid JID: %w", err)
	}

	return WithConnection(func(db *store.DB, client *whatsapp.Client) error {
		if err := client.WA.LeaveGroup(context.Background(), jid); err != nil {
			return fmt.Errorf("failed to leave group: %w", err)
		}

		return OutputResult(map[string]any{
			"jid": jid.String(),
		}, fmt.Sprintf("Left group %s", jid.String()))
	})
}

func runGroupsRename(cmd *cobra.Command, args []string) error {
	jid, err := types.ParseJID(args[0])
	if err != nil {
		return fmt.Errorf("invalid JID: %w", err)
	}
	name := args[1]

	return WithConnection(func(db *store.DB, client *whatsapp.Client) error {
		if err := client.WA.SetGroupName(context.Background(), jid, name); err != nil {
			return fmt.Errorf("failed to rename group: %w", err)
		}

		return OutputResult(map[string]any{
			"jid":  jid.String(),
			"name": name,
		}, fmt.Sprintf("Renamed group to '%s'", name))
	})
}
