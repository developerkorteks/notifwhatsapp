package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

const liveConfirmation = "I_UNDERSTAND_THIS_SENDS_TO_WHATSAPP"

func main() {
	var (
		sessionPath = flag.String("session", "", "path to a whatsmeow SQLite session")
		groupRaw    = flag.String("group", "", "target group JID")
		groupName   = flag.String("expect-name", "", "required exact group name")
		suite       = flag.String("suite", "full", "test suite: full, attribution, hidetag, swgc-hidetag, status-mention, swgc-group-mention, or swgc-then-group-mention")
		live        = flag.Bool("live", false, "send the test messages")
		confirm     = flag.String("confirm", "", "required confirmation for live mode")
	)
	flag.Parse()

	if *sessionPath == "" || *groupRaw == "" || *groupName == "" {
		flag.Usage()
		os.Exit(2)
	}
	if *live && *confirm != liveConfirmation {
		log.Fatalf("live mode requires -confirm=%s", liveConfirmation)
	}
	validSuites := map[string]bool{
		"full": true, "attribution": true, "hidetag": true,
		"swgc-hidetag": true, "status-mention": true,
		"swgc-group-mention": true, "swgc-then-group-mention": true,
		"swgc-hidetag-v2": true, "swgc-hidetag-v3": true,
		"swgc-mention-retry": true,
	}
	if !validSuites[*suite] {
		log.Fatalf("unknown suite %q", *suite)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	logger := waLog.Stdout("SWGC-LAB", "INFO", true)
	container, err := sqlstore.New(ctx, "sqlite3", "file:"+*sessionPath+"?_foreign_keys=on", logger)
	must("open session", err)
	device, err := container.GetFirstDevice(ctx)
	must("load device", err)
	if device.ID == nil {
		log.Fatal("session has no logged-in device")
	}

	client := whatsmeow.NewClient(device, logger)
	messageEvents := make(chan *events.Message, 32)
	client.AddEventHandler(func(event any) {
		if message, ok := event.(*events.Message); ok {
			messageEvents <- message
		}
	})
	must("connect", client.Connect())
	defer client.Disconnect()

	group, err := types.ParseJID(*groupRaw)
	must("parse group JID", err)
	if group.Server != types.GroupServer {
		log.Fatalf("target %s is not a group JID", group)
	}

	info, err := client.GetGroupInfo(ctx, group)
	must("get group info", err)
	if info.Name != *groupName {
		log.Fatalf("target guard failed: got group name %q, expected %q", info.Name, *groupName)
	}

	ownPN := device.GetJID().ToNonAD()
	ownLID := device.GetLID().ToNonAD()
	var admin types.JID
	var adminPN types.JID
	var otherMember types.JID
	var ownParticipant *types.GroupParticipant
	for i := range info.Participants {
		participant := &info.Participants[i]
		if admin.IsEmpty() && (participant.IsAdmin || participant.IsSuperAdmin) {
			admin = participant.JID.ToNonAD()
			adminPN = participant.PhoneNumber.ToNonAD()
		}
		if sameUser(participant.JID, ownPN) || sameUser(participant.JID, ownLID) ||
			sameUser(participant.PhoneNumber, ownPN) || sameUser(participant.LID, ownLID) {
			ownParticipant = participant
		} else if otherMember.IsEmpty() && !participant.IsAdmin && !participant.IsSuperAdmin {
			otherMember = participant.JID.ToNonAD()
		}
	}
	if ownParticipant == nil {
		log.Fatal("authenticated account was not found in the target group")
	}
	if admin.IsEmpty() {
		log.Fatal("no admin participant found")
	}
	if adminPN.IsEmpty() && admin.Server == types.DefaultUserServer {
		adminPN = admin
	}

	fmt.Printf("GROUP name=%q jid=%s announce_only=%t participants=%d\n",
		info.Name, info.JID, info.IsAnnounce, len(info.Participants))
	fmt.Printf("SESSION pn=%s lid=%s admin=%t super_admin=%t\n",
		ownPN, ownLID, ownParticipant.IsAdmin, ownParticipant.IsSuperAdmin)
	fmt.Printf("CLAIMED_ADMIN payload_only=%s\n", admin)
	if !adminPN.IsEmpty() {
		fmt.Printf("ADMIN_PN mention_target=%s\n", adminPN)
	}
	if otherMember.IsEmpty() {
		fmt.Println("CLAIMED_MEMBER payload_only=UNAVAILABLE")
	} else {
		fmt.Printf("CLAIMED_MEMBER payload_only=%s\n", otherMember)
	}

	if *suite == "status-mention" {
		runStatusMentionSuite(ctx, client, messageEvents, adminPN, *live)
		return
	}
	if *suite == "swgc-group-mention" {
		runSWGCGroupMentionSuite(ctx, client, messageEvents, group, info.Participants, *live)
		return
	}
	if *suite == "swgc-then-group-mention" {
		runSWGCThenGroupMentionSuite(ctx, client, messageEvents, group, info.Participants, *live)
		return
	}
	if *suite == "swgc-hidetag-v2" {
		runSWGCHidetagV2Suite(ctx, client, messageEvents, group, info.Participants, *live)
		return
	}
	if *suite == "swgc-hidetag-v3" {
		runSWGCHidetagV3Suite(ctx, client, messageEvents, group, info.Participants, *live)
		return
	}
	if *suite == "swgc-mention-retry" {
		runSWGCMentionRetry(ctx, client, messageEvents, group, info.Participants, *live)
		return
	}
	if !*live {
		fmt.Println("DRY_RUN no messages sent")
		return
	}
	if *suite == "attribution" {
		if otherMember.IsEmpty() {
			log.Fatal("attribution suite requires another non-admin member")
		}
		runAttributionSuite(ctx, client, messageEvents, group, otherMember)
		return
	}
	if *suite == "hidetag" {
		runHidetagSuite(ctx, client, messageEvents, group, info.Participants)
		return
	}
	if *suite == "swgc-hidetag" {
		runSWGCHidetagSuite(ctx, client, messageEvents, group, info.Participants)
		return
	}

	sendCase(ctx, client, messageEvents, group, "standard-control", &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("[SWGC LAB] standard control from authenticated member session"),
		},
	}, "")

	sendCase(ctx, client, messageEvents, group, "baseline", newSWGC(
		"[SWGC LAB] baseline from authenticated member session",
		nil,
	), "")

	claimedContext := &waE2E.ContextInfo{
		Participant:   proto.String(admin.String()),
		RemoteJID:     proto.String(group.String()),
		StanzaID:      proto.String("SWGC-LAB-NONEXISTENT-QUOTE"),
		IsGroupStatus: proto.Bool(true),
	}
	sendCase(ctx, client, messageEvents, group, "context-admin-claim", newSWGC(
		"[SWGC LAB] payload ContextInfo claims admin; transport sender must remain session member",
		claimedContext,
	), admin.String())

	statusQuoteClaim := newSWGC(
		"[SWGC LAB] StatusQuotedMessage key claims admin; transport sender must remain session member",
		nil,
	)
	statusQuoteClaim.GetGroupStatusMessageV2().Message.StatusQuotedMessage = &waE2E.StatusQuotedMessage{
		Type: waE2E.StatusQuotedMessage_QUESTION_ANSWER.Enum(),
		Text: proto.String("payload-only admin claim"),
		OriginalStatusID: &waCommon.MessageKey{
			RemoteJID:   proto.String(group.String()),
			FromMe:      proto.Bool(false),
			ID:          proto.String("SWGC-LAB-NONEXISTENT-STATUS"),
			Participant: proto.String(admin.String()),
		},
	}
	sendCase(ctx, client, messageEvents, group, "status-key-admin-claim", statusQuoteClaim, admin.String())

	if !otherMember.IsEmpty() {
		memberContext := &waE2E.ContextInfo{
			Participant:   proto.String(otherMember.String()),
			RemoteJID:     proto.String(group.String()),
			StanzaID:      proto.String("SWGC-LAB-NONEXISTENT-MEMBER-QUOTE"),
			IsGroupStatus: proto.Bool(true),
		}
		sendCase(ctx, client, messageEvents, group, "context-member-claim", newSWGC(
			"[SWGC LAB] payload ContextInfo claims another member; transport sender must remain session member",
			memberContext,
		), otherMember.String())

		memberStatusClaim := newSWGC(
			"[SWGC LAB] StatusQuotedMessage key claims another member; transport sender must remain session member",
			nil,
		)
		memberStatusClaim.GetGroupStatusMessageV2().Message.StatusQuotedMessage = &waE2E.StatusQuotedMessage{
			Type: waE2E.StatusQuotedMessage_QUESTION_ANSWER.Enum(),
			Text: proto.String("payload-only member claim"),
			OriginalStatusID: &waCommon.MessageKey{
				RemoteJID:   proto.String(group.String()),
				FromMe:      proto.Bool(false),
				ID:          proto.String("SWGC-LAB-NONEXISTENT-MEMBER-STATUS"),
				Participant: proto.String(otherMember.String()),
			},
		}
		sendCase(ctx, client, messageEvents, group, "status-key-member-claim", memberStatusClaim, otherMember.String())

		forwardClaim := newSWGC(
			"[SWGC LAB] forwarded/status attribution claims another member; transport sender must remain session member",
			&waE2E.ContextInfo{
				Participant:           proto.String(otherMember.String()),
				RemoteJID:             proto.String(group.String()),
				IsForwarded:           proto.Bool(true),
				ForwardingScore:       proto.Uint32(9),
				IsGroupStatus:         proto.Bool(true),
				ForwardOrigin:         waE2E.ContextInfo_STATUS.Enum(),
				StatusAttributionType: waE2E.ContextInfo_FORWARDED_FROM_STATUS.Enum(),
				StatusSourceType:      waE2E.ContextInfo_TEXT.Enum(),
			},
		)
		sendCase(ctx, client, messageEvents, group, "forward-attribution-member-claim", forwardClaim, otherMember.String())

		associationClaim := newSWGC(
			"[SWGC LAB] MessageAssociation parent claims another member; transport sender must remain session member",
			nil,
		)
		associationClaim.MessageContextInfo.MessageAssociation = &waE2E.MessageAssociation{
			AssociationType: waE2E.MessageAssociation_STATUS_EXTERNAL_RESHARE.Enum(),
			ParentMessageKey: &waCommon.MessageKey{
				RemoteJID:   proto.String(group.String()),
				FromMe:      proto.Bool(false),
				ID:          proto.String("SWGC-LAB-NONEXISTENT-ASSOCIATION"),
				Participant: proto.String(otherMember.String()),
			},
		}
		sendCase(ctx, client, messageEvents, group, "association-member-claim", associationClaim, otherMember.String())

		threadClaim := newSWGC(
			"[SWGC LAB] root meta thread sender claims another member; transport sender must remain session member",
			nil,
		)
		sendCaseExtra(
			ctx,
			client,
			messageEvents,
			group,
			"thread-meta-member-claim",
			threadClaim,
			otherMember.String(),
			whatsmeow.SendRequestExtra{
				Meta: &types.MsgMetaInfo{
					ThreadMessageID:        "SWGC-LAB-NONEXISTENT-THREAD",
					ThreadMessageSenderJID: otherMember,
				},
			},
		)

		deviceSentClaim := &waE2E.Message{
			DeviceSentMessage: &waE2E.DeviceSentMessage{
				DestinationJID: proto.String(otherMember.String()),
				Message: newSWGC(
					"[SWGC LAB] DeviceSentMessage destination claims another member; transport sender must remain session member",
					nil,
				),
			},
		}
		sendCase(ctx, client, messageEvents, group, "device-sent-member-claim", deviceSentClaim, otherMember.String())
	}

	newsletterClaim := newSWGC(
		"[SWGC LAB] forwarded newsletter attribution; transport sender must remain session member",
		&waE2E.ContextInfo{
			IsForwarded:     proto.Bool(true),
			ForwardingScore: proto.Uint32(1),
			ForwardOrigin:   waE2E.ContextInfo_CHANNELS.Enum(),
			ForwardedNewsletterMessageInfo: &waE2E.ContextInfo_ForwardedNewsletterMessageInfo{
				NewsletterJID:   proto.String("120363000000000000@newsletter"),
				ServerMessageID: proto.Int32(1),
				NewsletterName:  proto.String("SWGC LAB synthetic attribution"),
				ContentType:     waE2E.ContextInfo_ForwardedNewsletterMessageInfo_UPDATE.Enum(),
			},
		},
	)
	sendCase(ctx, client, messageEvents, group, "newsletter-attribution", newsletterClaim, "")
}

func runSWGCThenGroupMentionSuite(
	ctx context.Context,
	client *whatsmeow.Client,
	messageEvents <-chan *events.Message,
	group types.JID,
	participants []types.GroupParticipant,
	live bool,
) {
	mentionedJIDs := make([]string, 0, len(participants))
	for _, participant := range participants {
		target := participant.PhoneNumber.ToNonAD()
		if target.IsEmpty() {
			target = participant.JID.ToNonAD()
		}
		mentionedJIDs = append(mentionedJIDs, target.String())
	}

	fmt.Printf("SWGC_THEN_GROUP_MENTION targets=%d\n", len(mentionedJIDs))
	if !live {
		fmt.Println("DRY_RUN no SWGC or group mention sent")
		return
	}

	swgcResponse, err := client.SendMessage(ctx, group, newSWGC(
		"[SWGC LAB] SWGC diikuti mention seluruh anggota pada pesan grup",
		nil,
	))
	if err != nil {
		log.Fatalf("send SWGC before group mention: %v", err)
	}
	fmt.Printf("CASE name=swgc-before-group-mention result=ACK message_id=%s response_sender=%s timestamp=%s\n",
		swgcResponse.ID, swgcResponse.Sender, swgcResponse.Timestamp.Format(time.RFC3339))
	waitForEcho(messageEvents, swgcResponse.ID)

	mentionResponse, err := client.SendMessage(ctx, group, &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("[SWGC LAB MENTION] Notifikasi untuk seluruh anggota"),
			ContextInfo: &waE2E.ContextInfo{
				MentionedJID: mentionedJIDs,
			},
		},
	})
	if err != nil {
		log.Fatalf("send ordinary group mention: %v", err)
	}
	fmt.Printf("CASE name=ordinary-group-mention result=ACK message_id=%s response_sender=%s targets=%d timestamp=%s\n",
		mentionResponse.ID,
		mentionResponse.Sender,
		len(mentionedJIDs),
		mentionResponse.Timestamp.Format(time.RFC3339),
	)
	waitForEcho(messageEvents, mentionResponse.ID)
}

func runSWGCGroupMentionSuite(
	ctx context.Context,
	client *whatsmeow.Client,
	messageEvents <-chan *events.Message,
	group types.JID,
	participants []types.GroupParticipant,
	live bool,
) {
	mentionTargets := make([]types.JID, 0, len(participants))
	mentionedJIDs := make([]string, 0, len(participants))
	for _, participant := range participants {
		target := participant.PhoneNumber.ToNonAD()
		if target.IsEmpty() {
			target = participant.JID.ToNonAD()
		}
		mentionTargets = append(mentionTargets, target)
		mentionedJIDs = append(mentionedJIDs, target.String())
	}

	fmt.Printf("SWGC_GROUP_MENTION targets=%d\n", len(mentionTargets))
	for _, target := range mentionTargets {
		fmt.Printf("SWGC_GROUP_MENTION_TARGET jid=%s\n", target)
	}
	if !live {
		fmt.Println("DRY_RUN no SWGC or group-status mention notification sent")
		return
	}

	swgcMessage := newSWGC(
		"[SWGC LAB GROUP MENTION] Tes SWGC dengan tag seluruh anggota grup Manual",
		&waE2E.ContextInfo{MentionedJID: mentionedJIDs},
	)
	swgcResponse, err := client.SendMessage(
		ctx,
		group,
		swgcMessage,
	)
	if err != nil {
		log.Fatalf("send SWGC with group mention metadata: %v", err)
	}
	fmt.Printf("CASE name=swgc-with-all-member-mention result=ACK message_id=%s response_sender=%s timestamp=%s\n",
		swgcResponse.ID, swgcResponse.Sender, swgcResponse.Timestamp.Format(time.RFC3339))
	waitForEcho(messageEvents, swgcResponse.ID)

	groupMentionNotification := &waE2E.Message{
		GroupStatusMentionMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				ProtocolMessage: &waE2E.ProtocolMessage{
					Key: &waCommon.MessageKey{
						RemoteJID: proto.String(group.String()),
						FromMe:    proto.Bool(true),
						ID:        proto.String(swgcResponse.ID),
					},
					Type: waE2E.ProtocolMessage_STATUS_MENTION_MESSAGE.Enum(),
				},
			},
		},
	}
	ownPN := client.DangerousInternals().GetOwnID().ToNonAD()
	ownLID := client.DangerousInternals().GetOwnLID().ToNonAD()
	groupMentionNotification.GetGroupStatusMentionMessage().Message.ProtocolMessage.Key.Participant =
		proto.String(ownLID.String())
	mentionMetaNode := []waBinary.Node{{
		Tag:   "meta",
		Attrs: waBinary.Attrs{"is_status_mention": "true"},
	}}
	notificationCount := 0
	for _, target := range mentionTargets {
		if sameUser(target, ownPN) || sameUser(target, ownLID) {
			continue
		}
		notificationResponse, sendErr := client.SendMessage(
			ctx,
			target,
			groupMentionNotification,
			whatsmeow.SendRequestExtra{AdditionalNodes: &mentionMetaNode},
		)
		if sendErr != nil {
			fmt.Printf("CASE name=group-status-mention-notification result=ERROR target=%s error=%q\n",
				target, sendErr)
			continue
		}
		notificationCount++
		fmt.Printf("CASE name=group-status-mention-notification result=ACK message_id=%s response_sender=%s target=%s timestamp=%s\n",
			notificationResponse.ID,
			notificationResponse.Sender,
			target,
			notificationResponse.Timestamp.Format(time.RFC3339),
		)
		waitForEcho(messageEvents, notificationResponse.ID)
	}
	fmt.Printf("SWGC_GROUP_MENTION notifications_acked=%d notifications_expected=%d\n",
		notificationCount, len(mentionTargets)-1)
}

func runStatusMentionSuite(
	ctx context.Context,
	client *whatsmeow.Client,
	messageEvents <-chan *events.Message,
	adminPN types.JID,
	live bool,
) {
	if adminPN.IsEmpty() {
		log.Fatal("status mention requires the admin phone-number JID")
	}

	recipients, err := client.DangerousInternals().GetStatusBroadcastRecipients(ctx)
	must("get status broadcast recipients", err)
	adminInAudience := false
	for _, recipient := range recipients {
		if sameUser(recipient.ToNonAD(), adminPN) {
			adminInAudience = true
			break
		}
	}
	fmt.Printf("STATUS_MENTION target=%s audience=%d target_in_audience=%t\n",
		adminPN, len(recipients), adminInAudience)
	if !adminInAudience {
		log.Fatal("admin is not in the current WhatsApp Status privacy audience")
	}
	if !live {
		fmt.Println("DRY_RUN no status or mention notification sent")
		return
	}

	mentionedUsersNode := []waBinary.Node{{
		Tag:   "meta",
		Attrs: waBinary.Attrs{},
		Content: []waBinary.Node{{
			Tag:   "mentioned_users",
			Attrs: waBinary.Attrs{},
			Content: []waBinary.Node{{
				Tag:   "to",
				Attrs: waBinary.Attrs{"jid": adminPN},
			}},
		}},
	}}
	statusMessage := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text:           proto.String("[SWGC LAB STATUS MENTION] Tes mention admin grup Manual"),
			BackgroundArgb: proto.Uint32(0xFF0F8A5F),
			TextArgb:       proto.Uint32(0xFFFFFFFF),
			Font:           waE2E.ExtendedTextMessage_SYSTEM.Enum(),
			ContextInfo: &waE2E.ContextInfo{
				MentionedJID: []string{adminPN.String()},
			},
		},
	}
	statusResponse, err := client.SendMessage(
		ctx,
		types.StatusBroadcastJID,
		statusMessage,
		whatsmeow.SendRequestExtra{AdditionalNodes: &mentionedUsersNode},
	)
	if err != nil {
		log.Fatalf("send mentioned status: %v", err)
	}
	fmt.Printf("CASE name=status-with-admin-mention result=ACK message_id=%s response_sender=%s timestamp=%s\n",
		statusResponse.ID, statusResponse.Sender, statusResponse.Timestamp.Format(time.RFC3339))
	waitForEcho(messageEvents, statusResponse.ID)

	mentionNotification := &waE2E.Message{
		StatusMentionMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				ProtocolMessage: &waE2E.ProtocolMessage{
					Key: &waCommon.MessageKey{
						RemoteJID: proto.String(types.StatusBroadcastJID.String()),
						FromMe:    proto.Bool(true),
						ID:        proto.String(statusResponse.ID),
					},
					Type: waE2E.ProtocolMessage_STATUS_MENTION_MESSAGE.Enum(),
				},
			},
		},
	}
	mentionMetaNode := []waBinary.Node{{
		Tag:   "meta",
		Attrs: waBinary.Attrs{"is_status_mention": "true"},
	}}
	notificationResponse, err := client.SendMessage(
		ctx,
		adminPN,
		mentionNotification,
		whatsmeow.SendRequestExtra{AdditionalNodes: &mentionMetaNode},
	)
	if err != nil {
		log.Fatalf("send status mention notification: %v", err)
	}
	fmt.Printf("CASE name=status-mention-notification result=ACK message_id=%s response_sender=%s target=%s timestamp=%s\n",
		notificationResponse.ID,
		notificationResponse.Sender,
		adminPN,
		notificationResponse.Timestamp.Format(time.RFC3339),
	)
	waitForEcho(messageEvents, notificationResponse.ID)
}

func runHidetagSuite(
	ctx context.Context,
	client *whatsmeow.Client,
	messageEvents <-chan *events.Message,
	group types.JID,
	participants []types.GroupParticipant,
) {
	mentionedJIDs := make([]string, 0, len(participants))
	for _, participant := range participants {
		mentionedJIDs = append(mentionedJIDs, participant.JID.ToNonAD().String())
	}

	fmt.Printf("HIDETAG visible_tags=0 mentioned_jids=%d\n", len(mentionedJIDs))
	sendCase(ctx, client, messageEvents, group, "hidetag-all-participants", &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("[SWGC LAB HIDETAG] Tes mention metadata tanpa tag yang terlihat"),
			ContextInfo: &waE2E.ContextInfo{
				MentionedJID: mentionedJIDs,
			},
		},
	}, "")
}

func runSWGCHidetagSuite(
	ctx context.Context,
	client *whatsmeow.Client,
	messageEvents <-chan *events.Message,
	group types.JID,
	participants []types.GroupParticipant,
) {
	mentionedJIDs := make([]string, 0, len(participants))
	for _, participant := range participants {
		mentionedJIDs = append(mentionedJIDs, participant.JID.ToNonAD().String())
	}

	fmt.Printf("SWGC_HIDETAG visible_tags=0 mentioned_jids=%d\n", len(mentionedJIDs))
	sendCase(ctx, client, messageEvents, group, "swgc-hidetag-all-participants", newSWGC(
		"[SWGC LAB HIDETAG] Tes mention metadata di dalam SWGC tanpa tag yang terlihat",
		&waE2E.ContextInfo{
			MentionedJID: mentionedJIDs,
		},
	), "")
}

func runAttributionSuite(
	ctx context.Context,
	client *whatsmeow.Client,
	messageEvents <-chan *events.Message,
	group, otherMember types.JID,
) {
	quotedContext := &waE2E.ContextInfo{
		StanzaID:    proto.String("SWGC-LAB-SYNTHETIC-QUOTE"),
		Participant: proto.String(otherMember.String()),
		RemoteJID:   proto.String(group.String()),
		QuotedMessage: &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text: proto.String("Synthetic quoted message attributed to another member"),
			},
		},
		IsGroupStatus: proto.Bool(true),
	}
	sendCase(ctx, client, messageEvents, group, "quote-other-member", newSWGC(
		"[SWGC LAB ATTRIBUTION] Quote context references another member",
		quotedContext,
	), otherMember.String())

	forwardContext := &waE2E.ContextInfo{
		Participant:           proto.String(otherMember.String()),
		RemoteJID:             proto.String(group.String()),
		IsForwarded:           proto.Bool(true),
		ForwardingScore:       proto.Uint32(9),
		IsGroupStatus:         proto.Bool(true),
		ForwardOrigin:         waE2E.ContextInfo_STATUS.Enum(),
		StatusAttributionType: waE2E.ContextInfo_FORWARDED_FROM_STATUS.Enum(),
		StatusSourceType:      waE2E.ContextInfo_TEXT.Enum(),
	}
	sendCase(ctx, client, messageEvents, group, "forward-status-attribution", newSWGC(
		"[SWGC LAB ATTRIBUTION] Forwarded and status attribution",
		forwardContext,
	), otherMember.String())

	newsletterContext := &waE2E.ContextInfo{
		IsForwarded:     proto.Bool(true),
		ForwardingScore: proto.Uint32(1),
		ForwardOrigin:   waE2E.ContextInfo_CHANNELS.Enum(),
		ForwardedNewsletterMessageInfo: &waE2E.ContextInfo_ForwardedNewsletterMessageInfo{
			NewsletterJID:   proto.String("120363000000000000@newsletter"),
			ServerMessageID: proto.Int32(1),
			NewsletterName:  proto.String("SWGC LAB synthetic channel"),
			ContentType:     waE2E.ContextInfo_ForwardedNewsletterMessageInfo_UPDATE.Enum(),
		},
	}
	sendCase(ctx, client, messageEvents, group, "newsletter-attribution-only", newSWGC(
		"[SWGC LAB ATTRIBUTION] Synthetic channel/newsletter attribution",
		newsletterContext,
	), "")

	associationClaim := newSWGC(
		"[SWGC LAB ATTRIBUTION] Parent status metadata references another member",
		nil,
	)
	associationClaim.MessageContextInfo.MessageAssociation = &waE2E.MessageAssociation{
		AssociationType: waE2E.MessageAssociation_STATUS_EXTERNAL_RESHARE.Enum(),
		ParentMessageKey: &waCommon.MessageKey{
			RemoteJID:   proto.String(group.String()),
			FromMe:      proto.Bool(false),
			ID:          proto.String("SWGC-LAB-SYNTHETIC-PARENT-STATUS"),
			Participant: proto.String(otherMember.String()),
		},
	}
	sendCase(ctx, client, messageEvents, group, "parent-status-other-member", associationClaim, otherMember.String())
}

func newSWGC(text string, payloadContext *waE2E.ContextInfo) *waE2E.Message {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		log.Fatalf("generate message secret: %v", err)
	}

	inner := &waE2E.Message{
		MessageContextInfo: &waE2E.MessageContextInfo{MessageSecret: secret},
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text:           proto.String(text),
			BackgroundArgb: proto.Uint32(0xFF0F8A5F),
			TextArgb:       proto.Uint32(0xFFFFFFFF),
			Font:           waE2E.ExtendedTextMessage_SYSTEM.Enum(),
			ContextInfo:    payloadContext,
		},
	}
	return &waE2E.Message{
		MessageContextInfo: &waE2E.MessageContextInfo{MessageSecret: secret},
		GroupStatusMessageV2: &waE2E.FutureProofMessage{
			Message: inner,
		},
	}
}

func sendCase(
	ctx context.Context,
	client *whatsmeow.Client,
	messageEvents <-chan *events.Message,
	group types.JID,
	name string,
	message *waE2E.Message,
	claimedSender string,
) {
	sendCaseExtra(ctx, client, messageEvents, group, name, message, claimedSender, whatsmeow.SendRequestExtra{})
}

func sendCaseExtra(
	ctx context.Context,
	client *whatsmeow.Client,
	messageEvents <-chan *events.Message,
	group types.JID,
	name string,
	message *waE2E.Message,
	claimedSender string,
	extra whatsmeow.SendRequestExtra,
) {
	started := time.Now()
	response, err := client.SendMessage(ctx, group, message, extra)
	if err != nil {
		fmt.Printf("CASE name=%s result=ERROR duration=%s error=%q\n",
			name, time.Since(started).Round(time.Millisecond), err)
		return
	}
	fmt.Printf("CASE name=%s result=ACK duration=%s message_id=%s claimed_sender=%q response_sender=%s server_id=%d timestamp=%s\n",
		name,
		time.Since(started).Round(time.Millisecond),
		response.ID,
		claimedSender,
		response.Sender,
		response.ServerID,
		response.Timestamp.Format(time.RFC3339),
	)
	waitForEcho(messageEvents, response.ID)
}

func waitForEcho(messageEvents <-chan *events.Message, messageID types.MessageID) {
	timer := time.NewTimer(1500 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case event := <-messageEvents:
			if event.Info.ID != messageID {
				continue
			}
			fmt.Printf("ECHO message_id=%s chat=%s sender=%s sender_alt=%s from_me=%t swgc=%t\n",
				event.Info.ID,
				event.Info.Chat,
				event.Info.Sender,
				event.Info.SenderAlt,
				event.Info.IsFromMe,
				event.Message.GetGroupStatusMessageV2() != nil,
			)
			return
		case <-timer.C:
			fmt.Printf("ECHO message_id=%s result=NOT_OBSERVED\n", messageID)
			return
		}
	}
}

func runSWGCMentionRetry(
	ctx context.Context,
	client *whatsmeow.Client,
	messageEvents <-chan *events.Message,
	group types.JID,
	participants []types.GroupParticipant,
	live bool,
) {
	ownPN := client.DangerousInternals().GetOwnID().ToNonAD()
	ownLID := client.DangerousInternals().GetOwnLID().ToNonAD()

	mentionTargets := make([]types.JID, 0, len(participants))
	mentionedJIDs := make([]string, 0, len(participants))
	for _, participant := range participants {
		target := participant.PhoneNumber.ToNonAD()
		if target.IsEmpty() {
			target = participant.JID.ToNonAD()
		}
		mentionTargets = append(mentionTargets, target)
		mentionedJIDs = append(mentionedJIDs, target.String())
	}

	fmt.Printf("SWGC_MENTION_RETRY targets=%d\n", len(mentionTargets))
	if !live {
		fmt.Println("DRY_RUN no messages sent")
		return
	}

	// Kirim SWGC dengan MentionedJID di inner ContextInfo
	swgcMsg := newSWGC("Halo semua, cek update ini ya", &waE2E.ContextInfo{
		MentionedJID: mentionedJIDs,
	})
	resp, err := client.SendMessage(ctx, group, swgcMsg)
	if err != nil {
		log.Fatalf("send SWGC: %v", err)
	}
	fmt.Printf("SWGC sent id=%s sender=%s\n", resp.ID, resp.Sender)
	waitForEcho(messageEvents, resp.ID)

	// Tunggu 5 detik agar server proses SWGC dulu
	fmt.Println("Waiting 5s before sending mention notifications...")
	time.Sleep(5 * time.Second)

	// Kirim GroupStatusMentionMessage ke setiap member
	mentionMetaNode := []waBinary.Node{{
		Tag:   "meta",
		Attrs: waBinary.Attrs{"is_status_mention": "true"},
	}}
	notifMsg := &waE2E.Message{
		GroupStatusMentionMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				ProtocolMessage: &waE2E.ProtocolMessage{
					Key: &waCommon.MessageKey{
						RemoteJID:   proto.String(group.String()),
						FromMe:      proto.Bool(true),
						ID:          proto.String(resp.ID),
						Participant: proto.String(ownLID.String()),
					},
					Type: waE2E.ProtocolMessage_STATUS_MENTION_MESSAGE.Enum(),
				},
			},
		},
	}

	sentCount := 0
	for _, target := range mentionTargets {
		if sameUser(target, ownPN) || sameUser(target, ownLID) {
			continue
		}
		// Delay antar notif supaya tidak kena rate limit
		time.Sleep(2 * time.Second)
		nResp, sendErr := client.SendMessage(ctx, target, notifMsg,
			whatsmeow.SendRequestExtra{AdditionalNodes: &mentionMetaNode})
		if sendErr != nil {
			fmt.Printf("NOTIF ERROR target=%s err=%q\n", target, sendErr)
			continue
		}
		sentCount++
		fmt.Printf("NOTIF ACK target=%s id=%s\n", target, nResp.ID)
		waitForEcho(messageEvents, nResp.ID)
	}
	fmt.Printf("DONE sent=%d swgc_id=%s\n", sentCount, resp.ID)
}

func runSWGCHidetagV3Suite(
	ctx context.Context,
	client *whatsmeow.Client,
	messageEvents <-chan *events.Message,
	group types.JID,
	participants []types.GroupParticipant,
	live bool,
) {
	ownPN := client.DangerousInternals().GetOwnID().ToNonAD()
	ownLID := client.DangerousInternals().GetOwnLID().ToNonAD()

	mentionTargets := make([]types.JID, 0, len(participants))
	mentionedJIDs := make([]string, 0, len(participants))
	for _, participant := range participants {
		target := participant.PhoneNumber.ToNonAD()
		if target.IsEmpty() {
			target = participant.JID.ToNonAD()
		}
		mentionTargets = append(mentionTargets, target)
		mentionedJIDs = append(mentionedJIDs, target.String())
	}

	fmt.Printf("SWGC_HIDETAG_V3 targets=%d\n", len(mentionTargets))
	if !live {
		fmt.Println("DRY_RUN no messages sent")
		return
	}

	// --- Case A: Clean SWGC (NO MentionedJID) + only GroupStatusMentionMessage per member ---
	// Hipotesis: tanpa MentionedJID di SWGC, GroupStatusMentionMessage jadi satu-satunya signal
	// Ini harusnya render sebagai "mentioned you in their status update" bukan chat message
	fmt.Println("\n=== CASE A: Clean SWGC + GroupStatusMentionMessage only (no MentionedJID in SWGC) ===")

	swgcA := newSWGC("Update grup terbaru", nil)
	respA, err := client.SendMessage(ctx, group, swgcA)
	if err != nil {
		log.Fatalf("case A SWGC: %v", err)
	}
	fmt.Printf("CASE_A swgc=ACK id=%s sender=%s\n", respA.ID, respA.Sender)
	waitForEcho(messageEvents, respA.ID)

	mentionMetaNode := []waBinary.Node{{
		Tag:   "meta",
		Attrs: waBinary.Attrs{"is_status_mention": "true"},
	}}
	notifA := &waE2E.Message{
		GroupStatusMentionMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				ProtocolMessage: &waE2E.ProtocolMessage{
					Key: &waCommon.MessageKey{
						RemoteJID:   proto.String(group.String()),
						FromMe:      proto.Bool(true),
						ID:          proto.String(respA.ID),
						Participant: proto.String(ownLID.String()),
					},
					Type: waE2E.ProtocolMessage_STATUS_MENTION_MESSAGE.Enum(),
				},
			},
		},
	}
	for _, target := range mentionTargets {
		if sameUser(target, ownPN) || sameUser(target, ownLID) {
			continue
		}
		nResp, sendErr := client.SendMessage(ctx, target, notifA,
			whatsmeow.SendRequestExtra{AdditionalNodes: &mentionMetaNode})
		if sendErr != nil {
			fmt.Printf("CASE_A notif=ERROR target=%s err=%q\n", target, sendErr)
			continue
		}
		fmt.Printf("CASE_A notif=ACK target=%s id=%s\n", target, nResp.ID)
		waitForEcho(messageEvents, nResp.ID)
	}

	time.Sleep(3 * time.Second)

	// --- Case B: SWGC with MentionedJID ONLY (no separate GroupStatusMentionMessage) ---
	// Hipotesis: MentionedJID di inner ContextInfo saja sudah cukup trigger notif hidetag
	// seperti hidetag biasa di pesan grup
	fmt.Println("\n=== CASE B: SWGC with inner MentionedJID only (no separate notification) ===")

	swgcB := newSWGC("Info penting untuk semua anggota", &waE2E.ContextInfo{
		MentionedJID: mentionedJIDs,
	})
	respB, err := client.SendMessage(ctx, group, swgcB)
	if err != nil {
		fmt.Printf("CASE_B result=ERROR err=%q\n", err)
	} else {
		fmt.Printf("CASE_B swgc=ACK id=%s sender=%s\n", respB.ID, respB.Sender)
		waitForEcho(messageEvents, respB.ID)
	}

	time.Sleep(3 * time.Second)

	// --- Case C: SWGC with MentionedJID + GroupStatusMentionMessage (full combo) ---
	// Hipotesis: MentionedJID untuk trigger notif di group chat +
	// GroupStatusMentionMessage untuk trigger "mentioned you in status" di status tab
	fmt.Println("\n=== CASE C: SWGC(MentionedJID) + GroupStatusMentionMessage per member ===")

	swgcC := newSWGC("Pengumuman untuk seluruh member", &waE2E.ContextInfo{
		MentionedJID: mentionedJIDs,
	})
	respC, err := client.SendMessage(ctx, group, swgcC)
	if err != nil {
		log.Fatalf("case C SWGC: %v", err)
	}
	fmt.Printf("CASE_C swgc=ACK id=%s sender=%s\n", respC.ID, respC.Sender)
	waitForEcho(messageEvents, respC.ID)

	notifC := &waE2E.Message{
		GroupStatusMentionMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				ProtocolMessage: &waE2E.ProtocolMessage{
					Key: &waCommon.MessageKey{
						RemoteJID:   proto.String(group.String()),
						FromMe:      proto.Bool(true),
						ID:          proto.String(respC.ID),
						Participant: proto.String(ownLID.String()),
					},
					Type: waE2E.ProtocolMessage_STATUS_MENTION_MESSAGE.Enum(),
				},
			},
		},
	}
	for _, target := range mentionTargets {
		if sameUser(target, ownPN) || sameUser(target, ownLID) {
			continue
		}
		nResp, sendErr := client.SendMessage(ctx, target, notifC,
			whatsmeow.SendRequestExtra{AdditionalNodes: &mentionMetaNode})
		if sendErr != nil {
			fmt.Printf("CASE_C notif=ERROR target=%s err=%q\n", target, sendErr)
			continue
		}
		fmt.Printf("CASE_C notif=ACK target=%s id=%s\n", target, nResp.ID)
		waitForEcho(messageEvents, nResp.ID)
	}

	fmt.Println("\n=== DONE V3 ===")
	fmt.Println("Cek di device penerima:")
	fmt.Println("  Case A: Apakah muncul 'mentioned you in status' yang link ke SWGC?")
	fmt.Println("  Case B: Apakah ada notif mention di grup tanpa pesan tambahan?")
	fmt.Println("  Case C: Apakah ada notif mention + status mention link ke SWGC?")
}

func runSWGCHidetagV2Suite(
	ctx context.Context,
	client *whatsmeow.Client,
	messageEvents <-chan *events.Message,
	group types.JID,
	participants []types.GroupParticipant,
	live bool,
) {
	ownPN := client.DangerousInternals().GetOwnID().ToNonAD()
	ownLID := client.DangerousInternals().GetOwnLID().ToNonAD()

	mentionTargets := make([]types.JID, 0, len(participants))
	mentionedJIDs := make([]string, 0, len(participants))
	for _, participant := range participants {
		target := participant.PhoneNumber.ToNonAD()
		if target.IsEmpty() {
			target = participant.JID.ToNonAD()
		}
		mentionTargets = append(mentionTargets, target)
		mentionedJIDs = append(mentionedJIDs, target.String())
	}

	fmt.Printf("SWGC_HIDETAG_V2 targets=%d\n", len(mentionTargets))
	for _, t := range mentionTargets {
		fmt.Printf("  TARGET jid=%s\n", t)
	}
	if !live {
		fmt.Println("DRY_RUN no messages sent")
		return
	}

	// --- Case A: SWGC with MentionedJID in inner ContextInfo + GroupStatusMentionMessage per member ---
	fmt.Println("\n=== CASE A: SWGC(inner MentionedJID) + direct GroupStatusMentionMessage ===")

	swgcMsgA := newSWGC(
		"[SWGC LAB V2-A] SWGC hidetag dengan mention inner + notification langsung",
		&waE2E.ContextInfo{MentionedJID: mentionedJIDs},
	)
	respA, err := client.SendMessage(ctx, group, swgcMsgA)
	if err != nil {
		log.Fatalf("case A send SWGC: %v", err)
	}
	fmt.Printf("CASE_A swgc=ACK message_id=%s sender=%s timestamp=%s\n",
		respA.ID, respA.Sender, respA.Timestamp.Format(time.RFC3339))
	waitForEcho(messageEvents, respA.ID)

	groupMentionNotifA := &waE2E.Message{
		GroupStatusMentionMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				ProtocolMessage: &waE2E.ProtocolMessage{
					Key: &waCommon.MessageKey{
						RemoteJID:   proto.String(group.String()),
						FromMe:      proto.Bool(true),
						ID:          proto.String(respA.ID),
						Participant: proto.String(ownLID.String()),
					},
					Type: waE2E.ProtocolMessage_STATUS_MENTION_MESSAGE.Enum(),
				},
			},
		},
	}
	mentionMetaNode := []waBinary.Node{{
		Tag:   "meta",
		Attrs: waBinary.Attrs{"is_status_mention": "true"},
	}}
	notifCountA := 0
	for _, target := range mentionTargets {
		if sameUser(target, ownPN) || sameUser(target, ownLID) {
			continue
		}
		nResp, sendErr := client.SendMessage(ctx, target, groupMentionNotifA,
			whatsmeow.SendRequestExtra{AdditionalNodes: &mentionMetaNode})
		if sendErr != nil {
			fmt.Printf("CASE_A notif=ERROR target=%s error=%q\n", target, sendErr)
			continue
		}
		notifCountA++
		fmt.Printf("CASE_A notif=ACK target=%s message_id=%s\n", target, nResp.ID)
		waitForEcho(messageEvents, nResp.ID)
	}
	fmt.Printf("CASE_A notifications_sent=%d\n", notifCountA)

	time.Sleep(2 * time.Second)

	// --- Case B: groupMentionedMessage wrapper (field 62) ---
	fmt.Println("\n=== CASE B: groupMentionedMessage wrapper (field 62) ===")

	innerMsgB := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("[SWGC LAB V2-B] groupMentionedMessage wrapper test"),
			ContextInfo: &waE2E.ContextInfo{
				MentionedJID: mentionedJIDs,
			},
		},
	}
	msgB := &waE2E.Message{
		GroupMentionedMessage: &waE2E.FutureProofMessage{
			Message: innerMsgB,
		},
	}
	respB, err := client.SendMessage(ctx, group, msgB)
	if err != nil {
		fmt.Printf("CASE_B result=ERROR error=%q\n", err)
	} else {
		fmt.Printf("CASE_B result=ACK message_id=%s sender=%s timestamp=%s\n",
			respB.ID, respB.Sender, respB.Timestamp.Format(time.RFC3339))
		waitForEcho(messageEvents, respB.ID)
	}

	time.Sleep(2 * time.Second)

	// --- Case C: SWGC with MentionedJID at BOTH inner and outer level ---
	fmt.Println("\n=== CASE C: SWGC(inner + outer ContextInfo MentionedJID) ===")

	secret := make([]byte, 32)
	rand.Read(secret)
	innerC := &waE2E.Message{
		MessageContextInfo: &waE2E.MessageContextInfo{MessageSecret: secret},
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text:           proto.String("[SWGC LAB V2-C] Mention di inner dan juga groupMentionedMessage context"),
			BackgroundArgb: proto.Uint32(0xFF0F8A5F),
			TextArgb:       proto.Uint32(0xFFFFFFFF),
			Font:           waE2E.ExtendedTextMessage_SYSTEM.Enum(),
			ContextInfo: &waE2E.ContextInfo{
				MentionedJID:  mentionedJIDs,
				IsGroupStatus: proto.Bool(true),
			},
		},
	}
	msgC := &waE2E.Message{
		MessageContextInfo: &waE2E.MessageContextInfo{MessageSecret: secret},
		GroupStatusMessageV2: &waE2E.FutureProofMessage{
			Message: innerC,
		},
		GroupMentionedMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				ExtendedTextMessage: &waE2E.ExtendedTextMessage{
					Text: proto.String("[SWGC LAB V2-C] groupMentionedMessage sidecar"),
					ContextInfo: &waE2E.ContextInfo{
						MentionedJID: mentionedJIDs,
					},
				},
			},
		},
	}
	respC, err := client.SendMessage(ctx, group, msgC)
	if err != nil {
		fmt.Printf("CASE_C result=ERROR error=%q\n", err)
	} else {
		fmt.Printf("CASE_C result=ACK message_id=%s sender=%s timestamp=%s\n",
			respC.ID, respC.Sender, respC.Timestamp.Format(time.RFC3339))
		waitForEcho(messageEvents, respC.ID)
	}

	time.Sleep(2 * time.Second)

	// --- Case D: Plain SWGC + mentioned_jid AdditionalNodes (different from meta/mentioned_users) ---
	fmt.Println("\n=== CASE D: SWGC + AdditionalNodes mentioned_jid ===")

	mentionNodes := make([]waBinary.Node, 0, len(mentionTargets))
	for _, target := range mentionTargets {
		if sameUser(target, ownPN) || sameUser(target, ownLID) {
			continue
		}
		mentionNodes = append(mentionNodes, waBinary.Node{
			Tag:   "to",
			Attrs: waBinary.Attrs{"jid": target},
		})
	}
	additionalD := []waBinary.Node{{
		Tag:     "mentioned",
		Attrs:   waBinary.Attrs{},
		Content: mentionNodes,
	}}
	swgcMsgD := newSWGC(
		"[SWGC LAB V2-D] SWGC dengan mentioned node di stanza XML",
		&waE2E.ContextInfo{MentionedJID: mentionedJIDs},
	)
	respD, err := client.SendMessage(ctx, group, swgcMsgD,
		whatsmeow.SendRequestExtra{AdditionalNodes: &additionalD})
	if err != nil {
		fmt.Printf("CASE_D result=ERROR error=%q\n", err)
	} else {
		fmt.Printf("CASE_D result=ACK message_id=%s sender=%s timestamp=%s\n",
			respD.ID, respD.Sender, respD.Timestamp.Format(time.RFC3339))
		waitForEcho(messageEvents, respD.ID)
	}

	fmt.Println("\n=== DONE ===")
}

func sameUser(first, second types.JID) bool {
	return !first.IsEmpty() && !second.IsEmpty() &&
		first.User == second.User && first.Server == second.Server
}

func must(action string, err error) {
	if err != nil {
		log.Fatalf("%s: %v", action, err)
	}
}
