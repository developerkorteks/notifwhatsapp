package whatsapp

import (
	"context"
	"errors"
	"log"

	"juraganxl-notif/internal/db"
	"juraganxl-notif/internal/models"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	waSyncAction "go.mau.fi/whatsmeow/proto/waSyncAction"
	"go.mau.fi/whatsmeow/types"
)

// SetCloseFriendsList overwrites the logged-in account's WhatsApp close-friends
// list (status audience mode = CLOSE_FRIENDS) with the given JIDs.
//
// This is an account-global setting: after this call the account's status
// audience defaults to close-friends-only, and only the listed JIDs (plus the
// account itself) receive statuses / SWGC-CF messages that carry the
// CLOSE_FRIENDS audience metadata. There is no whatsmeow builder for this
// appstate mutation, so the patch format below is reverse-engineered and may
// need adjustment if WhatsApp changes the protocol.
func SetCloseFriendsList(accountID uint, jids []types.JID) error {
	client, ok := Clients[accountID]
	if !ok || client == nil || !client.IsConnected() {
		return errors.New("WhatsApp client is not connected")
	}

	seen := make(map[string]bool, len(jids))
	userJIDs := make([]string, 0, len(jids))
	for _, j := range jids {
		if j.IsEmpty() {
			continue
		}
		key := j.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		userJIDs = append(userJIDs, key)
	}

	mode := waSyncAction.StatusPrivacyAction_CLOSE_FRIENDS
	patch := appstate.PatchInfo{
		Type: appstate.WAPatchRegularHigh,
		Mutations: []appstate.MutationInfo{{
			// status_privacy lives in the regular_high collection. Version 2 is
			// the same value used by mute/star mutations; adjust if the server
			// rejects the patch.
			Index:   []string{appstate.IndexStatusPrivacy},
			Version: 2,
			Value: &waSyncAction.SyncActionValue{
				StatusPrivacy: &waSyncAction.StatusPrivacyAction{
					Mode:    &mode,
					UserJID: userJIDs,
				},
			},
		}},
	}

	if err := client.SendAppState(context.Background(), patch); err != nil {
		return err
	}
	log.Printf("[Account %d] Close-friends list updated with %d JIDs", accountID, len(userJIDs))
	return nil
}

// CollectGroupMembers returns the deduplicated union of participant JIDs across
// the given groups. Phone-number JIDs are preferred over LIDs for status
// privacy.
func CollectGroupMembers(client *whatsmeow.Client, groupJIDs []types.JID) []types.JID {
	seen := make(map[types.JID]bool)
	var out []types.JID
	for _, gJID := range groupJIDs {
		info, err := client.GetGroupInfo(context.Background(), gJID)
		if err != nil {
			log.Printf("[close-friends] Failed to fetch group info for %s: %v", gJID, err)
			continue
		}
		for _, p := range info.Participants {
			j := p.JID
			if !p.PhoneNumber.IsEmpty() {
				j = p.PhoneNumber
			}
			j = j.ToNonAD()
			if j.IsEmpty() || seen[j] {
				continue
			}
			seen[j] = true
			out = append(out, j)
		}
	}
	return out
}

// SyncCloseFriendsForGroups populates the account's close-friends list with the
// union of members from the given groups, so SWGC-CF messages (audience =
// CLOSE_FRIENDS) become visible to all of them.
func SyncCloseFriendsForGroups(accountID uint, groupJIDs []types.JID) error {
	client, ok := Clients[accountID]
	if !ok || client == nil || !client.IsConnected() {
		return errors.New("WhatsApp client is not connected")
	}
	members := CollectGroupMembers(client, groupJIDs)
	return SetCloseFriendsList(accountID, members)
}

// SyncCloseFriendsFromActiveGroups populates the account's close-friends list
// with the union of members from all custom-active groups. Useful to pre-fill
// the list before sending SWGC-CF, or to re-sync after group membership changes.
func SyncCloseFriendsFromActiveGroups(accountID uint) error {
	client, ok := Clients[accountID]
	if !ok || client == nil || !client.IsConnected() {
		return errors.New("WhatsApp client is not connected")
	}

	var groups []models.GroupTarget
	db.DB.Where("account_id = ? AND is_custom_active = ?", accountID, true).Find(&groups)

	var gJIDs []types.JID
	for _, g := range groups {
		if j, err := ParseJID(g.JID); err == nil {
			gJIDs = append(gJIDs, j)
		}
	}
	if len(gJIDs) == 0 {
		return errors.New("no custom-active groups found for this account")
	}
	members := CollectGroupMembers(client, gJIDs)
	return SetCloseFriendsList(accountID, members)
}

// ResetCloseFriendsList restores the account's status audience to the default
// (all contacts) and clears the close-friends list. Use this to roll back the
// side effects of SWGC-CF.
func ResetCloseFriendsList(accountID uint) error {
	client, ok := Clients[accountID]
	if !ok || client == nil || !client.IsConnected() {
		return errors.New("WhatsApp client is not connected")
	}

	mode := waSyncAction.StatusPrivacyAction_CONTACTS
	patch := appstate.PatchInfo{
		Type: appstate.WAPatchRegularHigh,
		Mutations: []appstate.MutationInfo{{
			Index:   []string{appstate.IndexStatusPrivacy},
			Version: 2,
			Value: &waSyncAction.SyncActionValue{
				StatusPrivacy: &waSyncAction.StatusPrivacyAction{
					Mode:    &mode,
					UserJID: []string{},
				},
			},
		}},
	}
	if err := client.SendAppState(context.Background(), patch); err != nil {
		return err
	}
	log.Printf("[Account %d] Close-friends list reset to default (contacts)", accountID)
	return nil
}
