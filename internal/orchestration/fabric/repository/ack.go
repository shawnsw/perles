package repository

import (
	"sync"
	"time"

	"github.com/zjrosen/perles/internal/orchestration/fabric/domain"
)

// MemoryAckRepository is an in-memory implementation of AckRepository.
type MemoryAckRepository struct {
	mu   sync.RWMutex
	acks map[string]*domain.Ack // key -> ack

	// Indexes for efficient lookups
	byAgent  map[string][]string // agentID -> list of ack keys
	byThread map[string][]string // threadID -> list of ack keys

	// Dependencies for GetUnacked
	depRepo         DependencyRepository
	threadRepo      ThreadRepository
	subRepo         SubscriptionRepository
	participantRepo ParticipantRepository
}

// NewMemoryAckRepository creates a new in-memory ack repository.
func NewMemoryAckRepository(depRepo DependencyRepository, threadRepo ThreadRepository, subRepo SubscriptionRepository) *MemoryAckRepository {
	return &MemoryAckRepository{
		acks:       make(map[string]*domain.Ack),
		byAgent:    make(map[string][]string),
		byThread:   make(map[string][]string),
		depRepo:    depRepo,
		threadRepo: threadRepo,
		subRepo:    subRepo,
	}
}

// SetParticipantRepository sets the participant repository for @here expansion.
// This is optional - if not set, @here mentions won't be expanded in GetUnacked.
func (r *MemoryAckRepository) SetParticipantRepository(repo ParticipantRepository) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.participantRepo = repo
}

// Ack marks message threads as acknowledged by an agent.
func (r *MemoryAckRepository) Ack(agentID string, threadIDs ...string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	for _, threadID := range threadIDs {
		ack := domain.Ack{
			ThreadID: threadID,
			AgentID:  agentID,
			AckedAt:  now,
		}
		key := ack.Key()

		if _, exists := r.acks[key]; exists {
			continue // Already acked
		}

		r.acks[key] = &ack
		r.byAgent[agentID] = append(r.byAgent[agentID], key)
		r.byThread[threadID] = append(r.byThread[threadID], key)
	}

	return nil
}

// IsAcked checks if an agent has acknowledged a message.
func (r *MemoryAckRepository) IsAcked(threadID, agentID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := threadID + ":" + agentID
	_, exists := r.acks[key]
	return exists, nil
}

// GetUnacked returns all unacked messages for an agent, grouped by channel.
// This includes top-level messages, replies that mention the agent, replies in
// threads the agent participates in, and replies in channels where the agent
// has a ModeAll subscription.
func (r *MemoryAckRepository) GetUnacked(agentID string) (map[string]UnackedSummary, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ackedSet := make(map[string]bool)
	for _, key := range r.byAgent[agentID] {
		if ack, exists := r.acks[key]; exists {
			ackedSet[ack.ThreadID] = true
		}
	}

	msgType := domain.ThreadMessage
	messages, err := r.threadRepo.List(ListOptions{Type: &msgType})
	if err != nil {
		return nil, err
	}

	result := make(map[string]UnackedSummary)

	for _, msg := range messages {
		if ackedSet[msg.ID] {
			continue
		}
		if msg.IsArchived() {
			continue
		}
		// Don't show messages the agent sent themselves
		if msg.CreatedBy == agentID {
			continue
		}

		// Check if this is a top-level message (has ChildOf → channel)
		channelID, err := r.getChannelForMessage(msg.ID)
		if err != nil {
			continue
		}

		// Determine if agent should see this message
		if channelID != "" {
			// Top-level message: show if agent is mentioned, participant, subscribed,
			// or if @here was used and agent is a fabric participant
			shouldShow := msg.HasMention(agentID) || msg.IsParticipant(agentID) || r.isSubscribed(agentID, channelID) || r.isHereMentionTarget(msg, agentID)
			if !shouldShow {
				continue
			}
		} else {
			// Reply: find the channel first (needed for subscription check)
			channelID = r.getChannelForReply(msg.ID)
			if channelID == "" {
				continue
			}
			// Show if mentioned, participant in root thread,
			// @here target, or subscribed to the channel with ModeAll
			shouldShow := msg.HasMention(agentID) ||
				r.isParticipantInThread(agentID, msg.ID) ||
				r.isHereMentionTarget(msg, agentID) ||
				r.isSubscribedAll(agentID, channelID)
			if !shouldShow {
				continue
			}
		}

		summary := result[channelID]
		summary.Count++
		summary.ThreadIDs = append(summary.ThreadIDs, msg.ID)
		result[channelID] = summary
	}

	return result, nil
}

// getChannelForReply traverses the reply chain to find the channel.
// Replies have ReplyTo → parent, and eventually a message has ChildOf → channel.
func (r *MemoryAckRepository) getChannelForReply(messageID string) string {
	visited := make(map[string]bool)
	current := messageID

	for range 10 { // Max depth to prevent infinite loops
		if visited[current] {
			return ""
		}
		visited[current] = true

		// First check if this message has a direct channel relationship
		channelID, _ := r.getChannelForMessage(current)
		if channelID != "" {
			return channelID
		}

		// Otherwise, find the parent via ReplyTo
		replyTo := domain.RelationReplyTo
		parents, err := r.depRepo.GetParents(current, &replyTo)
		if err != nil || len(parents) == 0 {
			return ""
		}

		current = parents[0].DependsOnID
	}

	return ""
}

// GetAckedThreadIDs returns all thread IDs that an agent has acknowledged.
func (r *MemoryAckRepository) GetAckedThreadIDs(agentID string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keys := r.byAgent[agentID]
	result := make([]string, 0, len(keys))

	for _, key := range keys {
		if ack, exists := r.acks[key]; exists {
			result = append(result, ack.ThreadID)
		}
	}

	return result, nil
}

func (r *MemoryAckRepository) getChannelForMessage(messageID string) (string, error) {
	relation := domain.RelationChildOf
	deps, err := r.depRepo.GetParents(messageID, &relation)
	if err != nil {
		return "", err
	}

	for _, dep := range deps {
		return dep.DependsOnID, nil
	}

	return "", nil
}

// isParticipantInThread checks if an agent is a participant in the root thread.
// With flat threading, all replies point to the same root, so we check participants there.
func (r *MemoryAckRepository) isParticipantInThread(agentID, replyID string) bool {
	// Find the root message via reply_to
	replyTo := domain.RelationReplyTo
	parents, err := r.depRepo.GetParents(replyID, &replyTo)
	if err != nil || len(parents) == 0 {
		return false
	}

	rootID := parents[0].DependsOnID
	root, err := r.threadRepo.Get(rootID)
	if err != nil {
		return false
	}

	return root.IsParticipant(agentID)
}

// isSubscribed checks if an agent is subscribed to a channel.
func (r *MemoryAckRepository) isSubscribed(agentID, channelID string) bool {
	if r.subRepo == nil {
		return false
	}
	sub, err := r.subRepo.Get(channelID, agentID)
	if err != nil || sub == nil {
		return false
	}
	return true
}

// isSubscribedAll checks if an agent is subscribed to a channel with ModeAll.
// ModeAll subscribers see all messages in the channel, including thread replies.
// ModeMentions subscribers only see messages that mention them (handled separately).
func (r *MemoryAckRepository) isSubscribedAll(agentID, channelID string) bool {
	if r.subRepo == nil {
		return false
	}
	sub, err := r.subRepo.Get(channelID, agentID)
	if err != nil || sub == nil {
		return false
	}
	return sub.Mode == domain.ModeAll
}

// isHereMentionTarget checks if the message has @here and the agent is a fabric participant.
// @here is a broadcast mention that should be visible to all registered participants.
func (r *MemoryAckRepository) isHereMentionTarget(msg domain.Thread, agentID string) bool {
	if r.participantRepo == nil {
		return false
	}
	if !msg.HasMention(domain.MentionHere) {
		return false
	}
	// Check if agent is a registered fabric participant
	participant, err := r.participantRepo.Get(agentID)
	return err == nil && participant != nil
}

var _ AckRepository = (*MemoryAckRepository)(nil)
