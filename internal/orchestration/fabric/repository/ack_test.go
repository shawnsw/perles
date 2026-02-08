package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zjrosen/perles/internal/orchestration/fabric/domain"
)

func setupAckTestRepos() (*MemoryAckRepository, *MemoryThreadRepository, *MemoryDependencyRepository, *MemorySubscriptionRepository) {
	threadRepo := NewMemoryThreadRepository()
	depRepo := NewMemoryDependencyRepository()
	subRepo := NewMemorySubscriptionRepository()
	ackRepo := NewMemoryAckRepository(depRepo, threadRepo, subRepo)
	return ackRepo, threadRepo, depRepo, subRepo
}

func TestMemoryAckRepository_Ack(t *testing.T) {
	ackRepo, _, _, _ := setupAckTestRepos()

	err := ackRepo.Ack("agent-1", "msg-1", "msg-2")
	require.NoError(t, err)

	// Idempotent
	err = ackRepo.Ack("agent-1", "msg-1")
	require.NoError(t, err)
}

func TestMemoryAckRepository_IsAcked(t *testing.T) {
	ackRepo, _, _, _ := setupAckTestRepos()

	isAcked, err := ackRepo.IsAcked("msg-1", "agent-1")
	require.NoError(t, err)
	require.False(t, isAcked)

	err = ackRepo.Ack("agent-1", "msg-1")
	require.NoError(t, err)

	isAcked, err = ackRepo.IsAcked("msg-1", "agent-1")
	require.NoError(t, err)
	require.True(t, isAcked)

	// Different agent not acked
	isAcked, err = ackRepo.IsAcked("msg-1", "agent-2")
	require.NoError(t, err)
	require.False(t, isAcked)
}

func TestMemoryAckRepository_GetAckedThreadIDs(t *testing.T) {
	ackRepo, _, _, _ := setupAckTestRepos()

	err := ackRepo.Ack("agent-1", "msg-1", "msg-2", "msg-3")
	require.NoError(t, err)

	ids, err := ackRepo.GetAckedThreadIDs("agent-1")
	require.NoError(t, err)
	require.Len(t, ids, 3)
	require.Contains(t, ids, "msg-1")
	require.Contains(t, ids, "msg-2")
	require.Contains(t, ids, "msg-3")
}

func TestMemoryAckRepository_GetUnacked(t *testing.T) {
	ackRepo, threadRepo, depRepo, subRepo := setupAckTestRepos()

	// Create a channel
	channel, err := threadRepo.Create(domain.Thread{
		Type: domain.ThreadChannel,
		Slug: "tasks",
	})
	require.NoError(t, err)

	// Subscribe agent-1 to the channel so they can see messages
	_, err = subRepo.Subscribe(channel.ID, "agent-1", domain.ModeAll)
	require.NoError(t, err)

	// Create messages in the channel
	msg1, err := threadRepo.Create(domain.Thread{
		Type:    domain.ThreadMessage,
		Content: "Message 1",
	})
	require.NoError(t, err)
	err = depRepo.Add(domain.NewDependency(msg1.ID, channel.ID, domain.RelationChildOf))
	require.NoError(t, err)

	msg2, err := threadRepo.Create(domain.Thread{
		Type:    domain.ThreadMessage,
		Content: "Message 2",
	})
	require.NoError(t, err)
	err = depRepo.Add(domain.NewDependency(msg2.ID, channel.ID, domain.RelationChildOf))
	require.NoError(t, err)

	// Initially both messages unacked
	unacked, err := ackRepo.GetUnacked("agent-1")
	require.NoError(t, err)
	require.Len(t, unacked, 1)
	require.Equal(t, 2, unacked[channel.ID].Count)

	// Ack one message
	err = ackRepo.Ack("agent-1", msg1.ID)
	require.NoError(t, err)

	unacked, err = ackRepo.GetUnacked("agent-1")
	require.NoError(t, err)
	require.Equal(t, 1, unacked[channel.ID].Count)
	require.Contains(t, unacked[channel.ID].ThreadIDs, msg2.ID)
}

func TestMemoryAckRepository_GetUnacked_RepliesWithMentions(t *testing.T) {
	ackRepo, threadRepo, depRepo, subRepo := setupAckTestRepos()

	// Create a channel
	channel, err := threadRepo.Create(domain.Thread{
		Type: domain.ThreadChannel,
		Slug: "tasks",
	})
	require.NoError(t, err)

	// Subscribe all agents to the channel so they can see top-level messages
	_, err = subRepo.Subscribe(channel.ID, "coordinator", domain.ModeAll)
	require.NoError(t, err)
	_, err = subRepo.Subscribe(channel.ID, "worker-1", domain.ModeAll)
	require.NoError(t, err)
	_, err = subRepo.Subscribe(channel.ID, "worker-2", domain.ModeAll)
	require.NoError(t, err)

	// Create a top-level message in the channel
	msg, err := threadRepo.Create(domain.Thread{
		Type:      domain.ThreadMessage,
		Content:   "Task assignment",
		CreatedBy: "coordinator",
	})
	require.NoError(t, err)
	err = depRepo.Add(domain.NewDependency(msg.ID, channel.ID, domain.RelationChildOf))
	require.NoError(t, err)

	// Create a reply that mentions the coordinator
	reply, err := threadRepo.Create(domain.Thread{
		Type:      domain.ThreadMessage,
		Content:   "Implementation complete @coordinator",
		CreatedBy: "worker-1",
		Mentions:  []string{"coordinator"},
	})
	require.NoError(t, err)
	err = depRepo.Add(domain.NewDependency(reply.ID, msg.ID, domain.RelationReplyTo))
	require.NoError(t, err)

	// Coordinator should only see the reply that mentions them (not their own message)
	unacked, err := ackRepo.GetUnacked("coordinator")
	require.NoError(t, err)
	require.Len(t, unacked, 1)
	require.Equal(t, 1, unacked[channel.ID].Count)
	require.Contains(t, unacked[channel.ID].ThreadIDs, reply.ID)

	// Worker-1 (who wrote the reply) should see the original message and NOT their own reply.
	// They are subscribed with ModeAll so they see all thread replies in the channel,
	// but the self-filter (CreatedBy == agentID) excludes their own reply.
	unacked, err = ackRepo.GetUnacked("worker-1")
	require.NoError(t, err)
	require.Equal(t, 1, unacked[channel.ID].Count)
	require.Contains(t, unacked[channel.ID].ThreadIDs, msg.ID)

	// Worker-2 (not mentioned) should see both the original message AND the reply
	// because they are subscribed with ModeAll
	unacked, err = ackRepo.GetUnacked("worker-2")
	require.NoError(t, err)
	require.Equal(t, 2, unacked[channel.ID].Count)
	require.Contains(t, unacked[channel.ID].ThreadIDs, msg.ID)
	require.Contains(t, unacked[channel.ID].ThreadIDs, reply.ID)
}

func TestMemoryAckRepository_GetUnacked_NestedReplies(t *testing.T) {
	ackRepo, threadRepo, depRepo, subRepo := setupAckTestRepos()

	// Create a channel
	channel, err := threadRepo.Create(domain.Thread{
		Type: domain.ThreadChannel,
		Slug: "tasks",
	})
	require.NoError(t, err)

	// Subscribe coordinator to the channel so they can see top-level messages
	_, err = subRepo.Subscribe(channel.ID, "coordinator", domain.ModeAll)
	require.NoError(t, err)

	// Create a top-level message
	msg, err := threadRepo.Create(domain.Thread{
		Type:      domain.ThreadMessage,
		Content:   "Original message",
		CreatedBy: "coordinator",
	})
	require.NoError(t, err)
	err = depRepo.Add(domain.NewDependency(msg.ID, channel.ID, domain.RelationChildOf))
	require.NoError(t, err)

	// Create a reply
	reply1, err := threadRepo.Create(domain.Thread{
		Type:      domain.ThreadMessage,
		Content:   "First reply",
		CreatedBy: "worker-1",
	})
	require.NoError(t, err)
	err = depRepo.Add(domain.NewDependency(reply1.ID, msg.ID, domain.RelationReplyTo))
	require.NoError(t, err)

	// Create a nested reply that mentions coordinator
	reply2, err := threadRepo.Create(domain.Thread{
		Type:      domain.ThreadMessage,
		Content:   "Nested reply @coordinator",
		CreatedBy: "worker-2",
		Mentions:  []string{"coordinator"},
	})
	require.NoError(t, err)
	err = depRepo.Add(domain.NewDependency(reply2.ID, reply1.ID, domain.RelationReplyTo))
	require.NoError(t, err)

	// Coordinator should see both replies (not their own top-level message).
	// reply1: visible via ModeAll subscription to the channel
	// reply2: visible via direct @mention AND ModeAll subscription
	unacked, err := ackRepo.GetUnacked("coordinator")
	require.NoError(t, err)
	require.Equal(t, 2, unacked[channel.ID].Count)
	require.Contains(t, unacked[channel.ID].ThreadIDs, reply1.ID)
	require.Contains(t, unacked[channel.ID].ThreadIDs, reply2.ID)
}

func TestMemoryAckRepository_GetUnacked_HereMention(t *testing.T) {
	ackRepo, threadRepo, depRepo, _ := setupAckTestRepos()

	// Create participant repository and wire it to ack repo
	participantRepo := NewMemoryParticipantRepository()
	ackRepo.SetParticipantRepository(participantRepo)

	// Create a channel
	channel, err := threadRepo.Create(domain.Thread{
		Type: domain.ThreadChannel,
		Slug: "tasks",
	})
	require.NoError(t, err)

	// Register some participants (simulates workers calling fabric_join)
	_, err = participantRepo.Join("worker-1", domain.RoleWorker)
	require.NoError(t, err)
	_, err = participantRepo.Join("worker-2", domain.RoleWorker)
	require.NoError(t, err)
	_, err = participantRepo.Join("coordinator", domain.RoleCoordinator)
	require.NoError(t, err)

	// Create a message with @here mention
	msg, err := threadRepo.Create(domain.Thread{
		Type:      domain.ThreadMessage,
		Content:   "@here All workers please check in",
		CreatedBy: "coordinator",
		Mentions:  []string{"here"}, // @here is stored as literal "here"
	})
	require.NoError(t, err)
	err = depRepo.Add(domain.NewDependency(msg.ID, channel.ID, domain.RelationChildOf))
	require.NoError(t, err)

	// Worker-1 should see the @here message (they are a fabric participant)
	unacked, err := ackRepo.GetUnacked("worker-1")
	require.NoError(t, err)
	require.Equal(t, 1, unacked[channel.ID].Count, "worker-1 should see @here message")
	require.Contains(t, unacked[channel.ID].ThreadIDs, msg.ID)

	// Worker-2 should also see the @here message
	unacked, err = ackRepo.GetUnacked("worker-2")
	require.NoError(t, err)
	require.Equal(t, 1, unacked[channel.ID].Count, "worker-2 should see @here message")
	require.Contains(t, unacked[channel.ID].ThreadIDs, msg.ID)

	// Non-participant (worker-3) should NOT see the @here message
	unacked, err = ackRepo.GetUnacked("worker-3")
	require.NoError(t, err)
	require.Empty(t, unacked, "worker-3 is not a participant - should not see @here message")
}

func TestMemoryAckRepository_GetUnacked_HereMentionInReply(t *testing.T) {
	ackRepo, threadRepo, depRepo, _ := setupAckTestRepos()

	// Create participant repository and wire it to ack repo
	participantRepo := NewMemoryParticipantRepository()
	ackRepo.SetParticipantRepository(participantRepo)

	// Create a channel
	channel, err := threadRepo.Create(domain.Thread{
		Type: domain.ThreadChannel,
		Slug: "tasks",
	})
	require.NoError(t, err)

	// Register participants
	_, err = participantRepo.Join("worker-1", domain.RoleWorker)
	require.NoError(t, err)
	_, err = participantRepo.Join("worker-2", domain.RoleWorker)
	require.NoError(t, err)

	// Create a root message (no @here)
	rootMsg, err := threadRepo.Create(domain.Thread{
		Type:      domain.ThreadMessage,
		Content:   "Discussion topic",
		CreatedBy: "coordinator",
	})
	require.NoError(t, err)
	err = depRepo.Add(domain.NewDependency(rootMsg.ID, channel.ID, domain.RelationChildOf))
	require.NoError(t, err)

	// Create a reply with @here mention
	reply, err := threadRepo.Create(domain.Thread{
		Type:      domain.ThreadMessage,
		Content:   "@here I need everyone's input",
		CreatedBy: "coordinator",
		Mentions:  []string{"here"},
	})
	require.NoError(t, err)
	err = depRepo.Add(domain.NewDependency(reply.ID, rootMsg.ID, domain.RelationReplyTo))
	require.NoError(t, err)

	// Worker-1 should see the @here reply (they are a participant)
	unacked, err := ackRepo.GetUnacked("worker-1")
	require.NoError(t, err)
	require.Equal(t, 1, unacked[channel.ID].Count, "worker-1 should see @here reply")
	require.Contains(t, unacked[channel.ID].ThreadIDs, reply.ID)

	// Non-participant should NOT see the @here reply
	unacked, err = ackRepo.GetUnacked("outside-agent")
	require.NoError(t, err)
	require.Empty(t, unacked, "outside-agent is not a participant - should not see @here reply")
}

func TestMemoryAckRepository_GetUnacked_ParticipantSeesReplies(t *testing.T) {
	ackRepo, threadRepo, depRepo, _ := setupAckTestRepos()

	// Create a channel
	channel, err := threadRepo.Create(domain.Thread{
		Type: domain.ThreadChannel,
		Slug: "general",
	})
	require.NoError(t, err)

	// Create a root message with participants (simulates coordinator @mentioning workers)
	root, err := threadRepo.Create(domain.Thread{
		Type:         domain.ThreadMessage,
		Content:      "Hey @worker-1 @worker-2 @worker-3 what's the best language?",
		CreatedBy:    "coordinator",
		Mentions:     []string{"worker-1", "worker-2", "worker-3"},
		Participants: []string{"coordinator", "worker-1", "worker-2", "worker-3"},
	})
	require.NoError(t, err)
	err = depRepo.Add(domain.NewDependency(root.ID, channel.ID, domain.RelationChildOf))
	require.NoError(t, err)

	// Worker-1 acks the root message
	err = ackRepo.Ack("worker-1", root.ID)
	require.NoError(t, err)

	// Worker-2 replies (with flat threading - reply points to root)
	reply1, err := threadRepo.Create(domain.Thread{
		Type:      domain.ThreadMessage,
		Content:   "I think Go is great!",
		CreatedBy: "worker-2",
	})
	require.NoError(t, err)
	err = depRepo.Add(domain.NewDependency(reply1.ID, root.ID, domain.RelationReplyTo))
	require.NoError(t, err)

	// Worker-3 also replies (no mention of worker-1)
	reply2, err := threadRepo.Create(domain.Thread{
		Type:      domain.ThreadMessage,
		Content:   "Rust is my favorite",
		CreatedBy: "worker-3",
	})
	require.NoError(t, err)
	err = depRepo.Add(domain.NewDependency(reply2.ID, root.ID, domain.RelationReplyTo))
	require.NoError(t, err)

	// Worker-1 should see both replies because they're a participant in the root thread
	// (even though neither reply @mentions worker-1)
	unacked, err := ackRepo.GetUnacked("worker-1")
	require.NoError(t, err)
	require.Equal(t, 2, unacked[channel.ID].Count, "worker-1 should see 2 replies as thread participant")
	require.Contains(t, unacked[channel.ID].ThreadIDs, reply1.ID)
	require.Contains(t, unacked[channel.ID].ThreadIDs, reply2.ID)

	// Worker-1 should NOT see the root (they already acked it)
	require.NotContains(t, unacked[channel.ID].ThreadIDs, root.ID)

	// A non-participant (worker-4) should not see anything:
	// - Not mentioned in root
	// - Not a participant
	// - Not subscribed to the channel
	unacked, err = ackRepo.GetUnacked("worker-4")
	require.NoError(t, err)
	require.Empty(t, unacked, "worker-4 is not mentioned/participant/subscribed - sees nothing")
}

func TestMemoryAckRepository_GetUnacked_ModeAllSubscriberSeesReplies(t *testing.T) {
	ackRepo, threadRepo, depRepo, subRepo := setupAckTestRepos()

	// Create a channel
	channel, err := threadRepo.Create(domain.Thread{
		Type: domain.ThreadChannel,
		Slug: "tasks",
	})
	require.NoError(t, err)

	// Observer subscribes with ModeAll — should see everything including thread replies
	_, err = subRepo.Subscribe(channel.ID, "observer", domain.ModeAll)
	require.NoError(t, err)

	// Coordinator posts a top-level message mentioning worker-1 only
	root, err := threadRepo.Create(domain.Thread{
		Type:         domain.ThreadMessage,
		Content:      "Hey @worker-1 implement the auth module",
		CreatedBy:    "coordinator",
		Mentions:     []string{"worker-1"},
		Participants: []string{"coordinator", "worker-1"},
	})
	require.NoError(t, err)
	err = depRepo.Add(domain.NewDependency(root.ID, channel.ID, domain.RelationChildOf))
	require.NoError(t, err)

	// Observer should see the top-level message (subscribed to channel)
	unacked, err := ackRepo.GetUnacked("observer")
	require.NoError(t, err)
	require.Equal(t, 1, unacked[channel.ID].Count)
	require.Contains(t, unacked[channel.ID].ThreadIDs, root.ID)

	// Observer ACKs the root message
	err = ackRepo.Ack("observer", root.ID)
	require.NoError(t, err)

	// Worker-1 replies (observer is NOT mentioned, NOT a participant on the thread)
	reply1, err := threadRepo.Create(domain.Thread{
		Type:      domain.ThreadMessage,
		Content:   "Auth module done, @coordinator please review",
		CreatedBy: "worker-1",
		Mentions:  []string{"coordinator"},
	})
	require.NoError(t, err)
	err = depRepo.Add(domain.NewDependency(reply1.ID, root.ID, domain.RelationReplyTo))
	require.NoError(t, err)

	// Coordinator replies back
	reply2, err := threadRepo.Create(domain.Thread{
		Type:      domain.ThreadMessage,
		Content:   "Looks good, merging",
		CreatedBy: "coordinator",
	})
	require.NoError(t, err)
	err = depRepo.Add(domain.NewDependency(reply2.ID, root.ID, domain.RelationReplyTo))
	require.NoError(t, err)

	// Observer should see both replies via ModeAll subscription,
	// even though they are not mentioned or a participant in the thread
	unacked, err = ackRepo.GetUnacked("observer")
	require.NoError(t, err)
	require.Equal(t, 2, unacked[channel.ID].Count, "observer with ModeAll should see all replies")
	require.Contains(t, unacked[channel.ID].ThreadIDs, reply1.ID)
	require.Contains(t, unacked[channel.ID].ThreadIDs, reply2.ID)

	// Root should NOT reappear (already ACK'd)
	require.NotContains(t, unacked[channel.ID].ThreadIDs, root.ID)
}

func TestMemoryAckRepository_GetUnacked_ModeMentionsSubscriberSkipsReplies(t *testing.T) {
	ackRepo, threadRepo, depRepo, subRepo := setupAckTestRepos()

	// Create a channel
	channel, err := threadRepo.Create(domain.Thread{
		Type: domain.ThreadChannel,
		Slug: "tasks",
	})
	require.NoError(t, err)

	// Agent subscribes with ModeMentions — should only see top-level messages
	// and replies that explicitly mention them
	_, err = subRepo.Subscribe(channel.ID, "watcher", domain.ModeMentions)
	require.NoError(t, err)

	// Coordinator posts a top-level message (watcher is NOT mentioned)
	root, err := threadRepo.Create(domain.Thread{
		Type:         domain.ThreadMessage,
		Content:      "Hey @worker-1 implement the auth module",
		CreatedBy:    "coordinator",
		Mentions:     []string{"worker-1"},
		Participants: []string{"coordinator", "worker-1"},
	})
	require.NoError(t, err)
	err = depRepo.Add(domain.NewDependency(root.ID, channel.ID, domain.RelationChildOf))
	require.NoError(t, err)

	// Watcher with ModeMentions should still see top-level messages
	// (isSubscribed is used for top-level, which doesn't filter by mode)
	unacked, err := ackRepo.GetUnacked("watcher")
	require.NoError(t, err)
	require.Equal(t, 1, unacked[channel.ID].Count, "ModeMentions subscriber sees top-level messages")

	// ACK the root
	err = ackRepo.Ack("watcher", root.ID)
	require.NoError(t, err)

	// Worker replies without mentioning watcher
	reply1, err := threadRepo.Create(domain.Thread{
		Type:      domain.ThreadMessage,
		Content:   "Done with auth",
		CreatedBy: "worker-1",
	})
	require.NoError(t, err)
	err = depRepo.Add(domain.NewDependency(reply1.ID, root.ID, domain.RelationReplyTo))
	require.NoError(t, err)

	// Watcher should NOT see the reply (ModeMentions, not mentioned, not participant)
	unacked, err = ackRepo.GetUnacked("watcher")
	require.NoError(t, err)
	require.Empty(t, unacked, "ModeMentions subscriber should not see replies without mention")

	// Now a reply that mentions watcher
	reply2, err := threadRepo.Create(domain.Thread{
		Type:      domain.ThreadMessage,
		Content:   "Hey @watcher can you review?",
		CreatedBy: "worker-1",
		Mentions:  []string{"watcher"},
	})
	require.NoError(t, err)
	err = depRepo.Add(domain.NewDependency(reply2.ID, root.ID, domain.RelationReplyTo))
	require.NoError(t, err)

	// Watcher SHOULD see this reply (they are mentioned)
	unacked, err = ackRepo.GetUnacked("watcher")
	require.NoError(t, err)
	require.Equal(t, 1, unacked[channel.ID].Count, "ModeMentions subscriber should see reply that mentions them")
	require.Contains(t, unacked[channel.ID].ThreadIDs, reply2.ID)
}
