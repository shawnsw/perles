import { useState, useMemo, useEffect, useCallback } from 'react'
import type { FabricEvent, Agent, AgentsResponse } from '../types'
import { hashColor } from '../utils/colors'
import ChatInput from './ChatInput'
import Markdown from './Markdown'
import Toast from './Toast'
import ReactionBar from './ReactionBar'
import ReactionTooltip from './ReactionTooltip'
import './FabricPanel.css'

interface Props {
  events: FabricEvent[]
  workflowId?: string
  sessionPath?: string
}

interface Channel {
  id: string
  slug: string
  title: string
  purpose: string
  createdAt: string
  messageCount: number
}

interface Message {
  id: string
  channelId: string
  parentId?: string
  createdBy: string
  content: string
  kind?: string
  timestamp: string
  type: 'message' | 'reply'
  mentions: string[]
  seq: number
}

interface Thread {
  parentMessage: Message
  replies: Message[]
  isOrphaned?: boolean
}

interface Artifact {
  id: string
  channelId: string
  name: string
  mediaType: string
  sizeBytes: number
  storageUri: string
  createdBy: string
  createdAt: string
}

interface Reaction {
  threadId: string
  agentId: string
  emoji: string
}

interface ReactionSummary {
  emoji: string
  count: number
  agentIds: string[]
}

function getInitialSidebarTab(): 'events' | 'messages' {
  const params = new URLSearchParams(window.location.search)
  const subtab = params.get('subtab')
  if (subtab === 'events' || subtab === 'messages') {
    return subtab
  }
  return 'messages'
}

function getInitialChannelId(): string | null {
  const params = new URLSearchParams(window.location.search)
  return params.get('channel')
}

function channelTitleForSlug(slug: string): string {
  const titles: Record<string, string> = {
    root: 'Root',
    system: 'System',
    tasks: 'Tasks',
    planning: 'Planning',
    general: 'General',
    observer: 'Observer',
  }

  if (titles[slug]) {
    return titles[slug]
  }

  return slug
    .split(/[-_]/g)
    .filter(Boolean)
    .map(part => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}

export default function FabricPanel({ events, workflowId, sessionPath }: Props) {
  const [selectedChannelId, setSelectedChannelIdState] = useState<string | null>(getInitialChannelId)
  const [selectedThread, setSelectedThread] = useState<Thread | null>(null)
  const [sidebarTab, setSidebarTabState] = useState<'events' | 'messages'>(getInitialSidebarTab)
  const [expandedEvents, setExpandedEvents] = useState<Set<number>>(new Set())
  // agents is fetched here for use by MentionAutocomplete
  const [agents, setAgents] = useState<Agent[]>([])
  const [isWorkflowActive, setIsWorkflowActive] = useState(false)
  const [canCompose, setCanCompose] = useState(Boolean(sessionPath))
  // Toast state for error/success notifications
  const [toast, setToast] = useState<{ message: string; type: 'error' | 'success' | 'info' } | null>(null)
  // Artifacts view state
  const [showArtifacts, setShowArtifacts] = useState(false)
  const [selectedArtifact, setSelectedArtifact] = useState<Artifact | null>(null)
  const [artifactContent, setArtifactContent] = useState<string | null>(null)

  const openThread = useCallback((thread: Thread) => {
    setSelectedThread(thread)
    setShowArtifacts(false)
    setSelectedArtifact(null)
  }, [])

  const setSidebarTab = (tab: 'events' | 'messages') => {
    setSidebarTabState(tab)
    const url = new URL(window.location.href)
    url.searchParams.set('subtab', tab)
    window.history.replaceState({}, '', url.toString())
  }

  const setSelectedChannelId = (channelId: string | null) => {
    setSelectedChannelIdState(channelId)
    const url = new URL(window.location.href)
    if (channelId) {
      url.searchParams.set('channel', channelId)
    } else {
      url.searchParams.delete('channel')
    }
    window.history.replaceState({}, '', url.toString())
  }

  // Extract channels, messages, artifacts, and reactions from events
  const { channels, messages, artifacts, reactions } = useMemo(() => {
    const channelMap = new Map<string, Channel>()
    const messageList: Message[] = []
    const artifactList: Artifact[] = []
    const reactionList: Reaction[] = []

    const ensureChannel = (
      channelId?: string,
      channelSlug?: string,
      seed?: Partial<Channel>
    ) => {
      if (!channelId || !channelSlug) {
        return
      }

      const existing = channelMap.get(channelId)
      const fallback: Channel = {
        id: channelId,
        slug: channelSlug,
        title: channelTitleForSlug(channelSlug),
        purpose: '',
        createdAt: seed?.createdAt || new Date().toISOString(),
        messageCount: 0,
      }

      channelMap.set(channelId, {
        ...fallback,
        ...existing,
        ...seed,
        id: channelId,
        slug: channelSlug,
      })
    }

    for (const event of events) {
      const e = event.event
      
      if (e.type === 'channel.created' && e.thread) {
        const slug = e.channel_slug || e.thread.slug || 'unknown'
        ensureChannel(e.channel_id, slug, {
          title: e.thread.title || channelTitleForSlug(slug),
          purpose: e.thread.purpose || '',
          createdAt: e.thread.created_at || event.timestamp,
        })
      }
      
      if ((e.type === 'message.posted' || e.type === 'reply.posted') && e.thread) {
        const channelId = e.channel_id || ''
        const channelSlug = e.channel_slug || channelMap.get(channelId)?.slug || 'unknown'
        ensureChannel(channelId, channelSlug, {
          createdAt: e.thread.created_at || event.timestamp,
        })

        const msg: Message = {
          id: e.thread.id,
          channelId,
          parentId: e.parent_id,
          createdBy: e.thread.created_by || 'unknown',
          content: e.thread.content || '',
          kind: e.thread.kind,
          timestamp: e.thread.created_at || event.timestamp,
          type: e.type === 'reply.posted' ? 'reply' : 'message',
          mentions: e.mentions || [],
          seq: e.thread.seq,
        }
        messageList.push(msg)
      }

      if (e.type === 'artifact.added' && e.thread && e.channel_id) {
        const channelSlug = e.channel_slug || channelMap.get(e.channel_id)?.slug || 'unknown'
        ensureChannel(e.channel_id, channelSlug, {
          createdAt: e.thread.created_at || event.timestamp,
        })
        artifactList.push({
          id: e.thread.id,
          channelId: e.channel_id,
          name: e.thread.name || 'Untitled',
          mediaType: e.thread.media_type || 'text/plain',
          sizeBytes: e.thread.size_bytes || 0,
          storageUri: e.thread.storage_uri || '',
          createdBy: e.thread.created_by || 'unknown',
          createdAt: e.thread.created_at || event.timestamp,
        })
      }

      if (e.type === 'reaction.added' && e.reaction) {
        reactionList.push({
          threadId: e.reaction.thread_id,
          agentId: e.reaction.agent_id,
          emoji: e.reaction.emoji,
        })
      }

      if (e.type === 'reaction.removed' && e.reaction) {
        // Remove the reaction from the list
        const idx = reactionList.findIndex(
          r => r.threadId === e.reaction!.thread_id &&
               r.agentId === e.reaction!.agent_id &&
               r.emoji === e.reaction!.emoji
        )
        if (idx !== -1) {
          reactionList.splice(idx, 1)
        }
      }
    }

    const messageCounts = new Map<string, number>()
    for (const msg of messageList) {
      if (!msg.channelId) continue
      messageCounts.set(msg.channelId, (messageCounts.get(msg.channelId) || 0) + 1)
    }

    for (const [channelId, count] of messageCounts.entries()) {
      const channel = channelMap.get(channelId)
      if (channel) {
        channel.messageCount = count
      }
    }

    const sortedChannels = Array.from(channelMap.values()).sort((a, b) => {
      const order = ['root', 'system', 'tasks', 'planning', 'general']
      const aIdx = order.indexOf(a.slug)
      const bIdx = order.indexOf(b.slug)
      if (aIdx !== -1 && bIdx !== -1) return aIdx - bIdx
      if (aIdx !== -1) return -1
      if (bIdx !== -1) return 1
      return a.slug.localeCompare(b.slug)
    })

    return { channels: sortedChannels, messages: messageList, artifacts: artifactList, reactions: reactionList }
  }, [events])

  // Helper to get reaction summaries for a message
  const getReactionSummaries = useCallback((messageId: string): ReactionSummary[] => {
    const messageReactions = reactions.filter(r => r.threadId === messageId)
    const emojiMap = new Map<string, string[]>()
    
    for (const reaction of messageReactions) {
      const agents = emojiMap.get(reaction.emoji) || []
      agents.push(reaction.agentId)
      emojiMap.set(reaction.emoji, agents)
    }
    
    return Array.from(emojiMap.entries()).map(([emoji, agentIds]) => ({
      emoji,
      count: agentIds.length,
      agentIds,
    }))
  }, [reactions])

  // Helper to format reaction tooltip content
  const formatReactionNames = (summary: ReactionSummary): string => {
    const names = summary.agentIds.map(id => id === 'user' ? 'You' : id)
    if (names.length === 1) return names[0]
    if (names.length === 2) return `${names[0]} and ${names[1]}`
    return `${names.slice(0, -1).join(', ')}, and ${names[names.length - 1]}`
  }

  // Group messages into threads using parent_id
  const threads = useMemo(() => {
    if (!selectedChannelId) return []
    
    // Build a map of ALL messages for parent lookup
    const allMessageMap = new Map<string, Message>()
    for (const msg of messages) {
      allMessageMap.set(msg.id, msg)
    }
    
    // Helper to find root channel for a message (walk up parent chain)
    const getRootChannelId = (msg: Message): string | undefined => {
      if (msg.channelId) return msg.channelId
      if (msg.parentId) {
        const parent = allMessageMap.get(msg.parentId)
        if (parent) return getRootChannelId(parent)
      }
      return undefined
    }
    
    // Get all messages belonging to this channel (including nested replies)
    const channelMessages = messages
      .filter(m => getRootChannelId(m) === selectedChannelId)
      .sort((a, b) => a.seq - b.seq)

    // Group replies by their direct parent_id
    const repliesByParent = new Map<string, Message[]>()
    const parentMessages: Message[] = []
    const orphanReplies: Message[] = []

    for (const msg of channelMessages) {
      if (msg.type === 'reply' && msg.parentId) {
        const parent = allMessageMap.get(msg.parentId)
        if (parent) {
          // This is a reply with a known parent
          const replies = repliesByParent.get(msg.parentId) || []
          replies.push(msg)
          repliesByParent.set(msg.parentId, replies)
        } else {
          // Sessions can contain replies without the original parent message.
          // Render them as standalone threads instead of dropping them silently.
          orphanReplies.push(msg)
        }
      } else {
        // This is a parent message (or a reply without parent_id, treat as parent)
        parentMessages.push(msg)
      }
    }
    
    // Recursively collect all descendant replies for a message
    const collectAllReplies = (messageId: string): Message[] => {
      const directReplies = repliesByParent.get(messageId) || []
      const allReplies: Message[] = []
      for (const reply of directReplies) {
        allReplies.push(reply)
        allReplies.push(...collectAllReplies(reply.id))
      }
      return allReplies
    }
    
    // Build threads from parent messages with all nested replies flattened
    const threadList: Thread[] = parentMessages.map(parent => ({
      parentMessage: parent,
      replies: collectAllReplies(parent.id).sort((a, b) => a.seq - b.seq),
      isOrphaned: false,
    }))

    const orphanThreadList: Thread[] = orphanReplies.map(reply => ({
      parentMessage: reply,
      replies: collectAllReplies(reply.id).sort((a, b) => a.seq - b.seq),
      isOrphaned: true,
    }))

    return [...threadList, ...orphanThreadList].sort(
      (a, b) => a.parentMessage.seq - b.parentMessage.seq
    )
  }, [messages, selectedChannelId])

  const selectedChannel = channels.find(c => c.id === selectedChannelId)

  // Get artifacts for selected channel
  const channelArtifacts = useMemo(() => {
    if (!selectedChannelId) return []
    return artifacts.filter(a => a.channelId === selectedChannelId)
  }, [artifacts, selectedChannelId])

  // Fetch artifact content when selected
  useEffect(() => {
    if (!selectedArtifact) {
      setArtifactContent(null)
      return
    }
    // Convert file:// URI to API path
    const filePath = selectedArtifact.storageUri.replace('file://', '')
    fetch(`/api/file?path=${encodeURIComponent(filePath)}`)
      .then(res => res.text())
      .then(content => setArtifactContent(content))
      .catch(() => setArtifactContent('Error loading artifact'))
  }, [selectedArtifact])

  // Auto-select first channel with messages (only if no valid channel selected)
  useEffect(() => {
    if (channels.length > 0 && !selectedChannel) {
      const firstWithMessages = channels.find(c => c.messageCount > 0) || channels[0]
      setSelectedChannelId(firstWithMessages.id)
    }
  }, [channels, selectedChannel])

  // Sync selectedThread with updated threads data when events change
  useEffect(() => {
    if (selectedThread) {
      const updatedThread = threads.find(t => t.parentMessage.id === selectedThread.parentMessage.id)
      if (updatedThread) {
        // Update with fresh data (new replies, etc.)
        setSelectedThread(updatedThread)
      } else {
        // Thread no longer exists, deselect
        setSelectedThread(null)
      }
    }
  }, [threads])

  // Fetch agents when the live workflow or archived session changes
  useEffect(() => {
    if (workflowId || sessionPath) {
      const params = new URLSearchParams()
      if (workflowId) {
        params.set('workflowId', workflowId)
      }
      if (sessionPath) {
        params.set('sessionPath', sessionPath)
      }

      fetch(`/api/fabric/agents?${params.toString()}`)
        .then(res => {
          if (!res.ok) throw new Error('Failed to fetch agents')
          return res.json()
        })
        .then((data: AgentsResponse) => {
          setAgents(data.agents || [])
          setIsWorkflowActive(data.isActive)
          setCanCompose(data.isActive || Boolean(sessionPath))
        })
        .catch(() => {
          setAgents([])
          setIsWorkflowActive(false)
          setCanCompose(false)
        })
    } else {
      setAgents([])
      setIsWorkflowActive(false)
      setCanCompose(false)
    }
  }, [workflowId, sessionPath])

  // Handle sending a new message to a channel
  const handleChannelSend = useCallback(async (content: string, mentions: string[]) => {
    if ((!workflowId && !sessionPath) || !selectedChannel) {
      const error = new Error('No session or channel selected')
      setToast({ message: error.message, type: 'error' })
      throw error
    }
    try {
      const res = await fetch('/api/fabric/send-message', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          workflowId,
          sessionPath,
          channelSlug: selectedChannel.slug,
          content,
          mentions
        })
      })
      if (!res.ok) {
        const data = await res.json()
        const errorMessage = data.error || 'Failed to send message'
        setToast({ message: errorMessage, type: 'error' })
        throw new Error(errorMessage)
      }
    } catch (error) {
      // Handle network errors (fetch failed)
      if (error instanceof TypeError && error.message.includes('fetch')) {
        setToast({ message: 'Network error. Please try again.', type: 'error' })
      } else if (error instanceof Error && !toast) {
        // Only show toast if not already set by the !res.ok block
        setToast({ message: error.message, type: 'error' })
      }
      throw error
    }
  }, [workflowId, sessionPath, selectedChannel, toast])

  // Handle sending a reply to a thread
  const handleThreadReply = useCallback(async (content: string, mentions: string[]) => {
    if ((!workflowId && !sessionPath) || !selectedThread) {
      const error = new Error('No session or thread selected')
      setToast({ message: error.message, type: 'error' })
      throw error
    }
    try {
      const res = await fetch('/api/fabric/reply', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          workflowId,
          sessionPath,
          threadId: selectedThread.parentMessage.id,
          content,
          mentions
        })
      })
      if (!res.ok) {
        const data = await res.json()
        const errorMessage = data.error || 'Failed to send reply'
        setToast({ message: errorMessage, type: 'error' })
        throw new Error(errorMessage)
      }
    } catch (error) {
      // Handle network errors (fetch failed)
      if (error instanceof TypeError && error.message.includes('fetch')) {
        setToast({ message: 'Network error. Please try again.', type: 'error' })
      } else if (error instanceof Error && !toast) {
        // Only show toast if not already set by the !res.ok block
        setToast({ message: error.message, type: 'error' })
      }
      throw error
    }
  }, [workflowId, sessionPath, selectedThread, toast])

  // Handle adding/removing a reaction
  const handleReact = useCallback(async (messageId: string, emoji: string, remove: boolean) => {
    if (!workflowId) {
      throw new Error('No active workflow')
    }
    try {
      const res = await fetch('/api/fabric/react', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          workflowId,
          messageId,
          emoji,
          remove,
        })
      })
      if (!res.ok) {
        const data = await res.json()
        const errorMessage = data.error || 'Failed to react'
        setToast({ message: errorMessage, type: 'error' })
        throw new Error(errorMessage)
      }
    } catch (error) {
      if (error instanceof TypeError && error.message.includes('fetch')) {
        setToast({ message: 'Network error. Please try again.', type: 'error' })
      }
      throw error
    }
  }, [workflowId])

  const formatTime = (ts: string) => {
    const date = new Date(ts)
    return date.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })
  }

  const formatDate = (ts: string) => {
    const date = new Date(ts)
    return date.toLocaleDateString([], { month: 'short', day: 'numeric' })
  }

  const formatBytes = (bytes: number): string => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
  }

  const getAgentColor = (agent: string): string => {
    return hashColor(agent)
  }

  const getChannelIcon = (slug: string): string => {
    const icons: Record<string, string> = {
      'root': '🏠',
      'system': '⚙️',
      'tasks': '📋',
      'planning': '📐',
      'general': '💬',
      'observer': '🔍',
    }
    return icons[slug] || '#'
  }

  // NOTE: Event types are defined in internal/orchestration/fabric/events.go
  // When adding new event types, update both events.go and this color map
  const getEventColor = (type: string): string => {
    const colors: Record<string, string> = {
      // Existing
      'channel.created': 'var(--accent-purple)',
      'message.posted': 'var(--accent-blue)',
      'reply.posted': 'var(--accent-green)',
      'acked': 'var(--accent-yellow)',
      'subscribed': 'var(--accent-orange)',
      // New - additions (green/purple/yellow)
      'participant.joined': 'var(--accent-green)',
      'artifact.added': 'var(--accent-purple)',
      'reaction.added': 'var(--accent-yellow)',
      // New - removals (red)
      'participant.left': 'var(--accent-red)',
      'reaction.removed': 'var(--accent-red)',
      'unsubscribed': 'var(--accent-red)',
      'channel.archived': 'var(--accent-red)',
    }
    return colors[type] || 'var(--text-muted)'
  }

  const getReplyAvatars = (replies: Message[]) => {
    const authors = [...new Set(replies.map(r => r.createdBy))]
    return authors.slice(0, 3)
  }

  return (
    <div className="fabric-panel-slack">
      {/* Channel Sidebar */}
      <aside className="channel-sidebar">
        <nav className="channel-list">
          {channels.map(channel => (
            <button
              key={channel.id}
              className={`channel-item ${sidebarTab === 'messages' && selectedChannelId === channel.id ? 'active' : ''}`}
              onClick={() => {
                setSidebarTab('messages')
                setSelectedChannelId(channel.id)
                setSelectedThread(null)
                setShowArtifacts(false)
                setSelectedArtifact(null)
              }}
            >
              <span className="channel-icon">{getChannelIcon(channel.slug)}</span>
              <span className="channel-name">{channel.title}</span>
              {channel.messageCount > 0 && (
                <span className="channel-badge">{channel.messageCount}</span>
              )}
            </button>
          ))}
        </nav>
        
        <div className="sidebar-footer">
          <button
            className={`channel-item events-item ${sidebarTab === 'events' ? 'active' : ''}`}
            onClick={() => {
              setSidebarTab('events')
              setSelectedThread(null)
            }}
          >
            <span className="channel-icon">📊</span>
            <span className="channel-name">Events</span>
            <span className="channel-badge">{events.length}</span>
          </button>
        </div>
      </aside>

      {/* Message Area */}
      <main className={`message-area ${selectedThread ? 'with-thread' : ''}`}>
        {sidebarTab === 'messages' ? (
          selectedChannel ? (
            <>
              <header className="channel-header">
                <div className="channel-header-top">
                  <span className="channel-icon-large">{getChannelIcon(selectedChannel.slug)}</span>
                  <div className="channel-title">
                    <h2>{selectedChannel.title}</h2>
                    {selectedChannel.purpose && (
                      <p className="channel-purpose">{selectedChannel.purpose}</p>
                    )}
                  </div>
                </div>
                <nav className="channel-tabs">
                  <button
                    className={`channel-tab ${!showArtifacts ? 'active' : ''}`}
                    onClick={() => {
                      setShowArtifacts(false)
                      setSelectedArtifact(null)
                    }}
                  >
                    <span className="channel-tab-icon">💬</span>
                    Messages
                  </button>
                  {channelArtifacts.length > 0 && (
                    <button
                      className={`channel-tab ${showArtifacts ? 'active' : ''}`}
                      onClick={() => setShowArtifacts(true)}
                    >
                      <span className="channel-tab-icon">📎</span>
                      Files
                      <span className="channel-tab-badge">{channelArtifacts.length}</span>
                    </button>
                  )}
                </nav>
              </header>

              {showArtifacts ? (
                <div className="artifacts-view">
                  <div className="artifacts-list">
                    {channelArtifacts.map(artifact => (
                      <button
                        key={artifact.id}
                        className={`artifact-item ${selectedArtifact?.id === artifact.id ? 'active' : ''}`}
                        onClick={() => setSelectedArtifact(artifact)}
                      >
                        <span className="artifact-icon">📄</span>
                        <div className="artifact-info">
                          <span className="artifact-name">{artifact.name}</span>
                          <span className="artifact-meta">
                            {artifact.createdBy} • {formatBytes(artifact.sizeBytes)}
                          </span>
                        </div>
                      </button>
                    ))}
                  </div>
                  <div className="artifact-content">
                    {selectedArtifact ? (
                      <>
                        <div className="artifact-content-header">
                          <h3>{selectedArtifact.name}</h3>
                          <span className="artifact-content-meta">
                            {selectedArtifact.mediaType} • {formatBytes(selectedArtifact.sizeBytes)}
                          </span>
                        </div>
                        {selectedArtifact.mediaType === 'text/markdown' ? (
                          <div className="artifact-content-body artifact-markdown">
                            {artifactContent ? (
                              <Markdown content={artifactContent} />
                            ) : (
                              <p>Loading...</p>
                            )}
                          </div>
                        ) : (
                          <pre className="artifact-content-body">
                            {artifactContent || 'Loading...'}
                          </pre>
                        )}
                      </>
                    ) : (
                      <div className="artifact-placeholder">
                        <p>Select an artifact to view its contents</p>
                      </div>
                    )}
                  </div>
                </div>
              ) : (
                <>
                  <div className="message-list">
                    {threads.length === 0 ? (
                      <div className="empty-channel">
                        <p>No messages in this channel</p>
                      </div>
                    ) : (
                      threads.map((thread) => {
                        const msgReactions = getReactionSummaries(thread.parentMessage.id)
                        return (
                        <div
                          key={thread.parentMessage.id}
                          className={`message-item ${selectedThread?.parentMessage.id === thread.parentMessage.id ? 'selected' : ''}`}
                        >
                          {/* Hover reaction bar */}
                          {isWorkflowActive && (
                            <ReactionBar
                              messageId={thread.parentMessage.id}
                              reactions={msgReactions}
                              onReact={(emoji, remove) => handleReact(thread.parentMessage.id, emoji, remove)}
                            />
                          )}

                          <div className="message-avatar" style={{ background: getAgentColor(thread.parentMessage.createdBy) }}>
                            {thread.parentMessage.createdBy.charAt(0).toUpperCase()}
                          </div>
                          <div className="message-body">
                            <div className="message-header">
                              <span className="message-author" style={{ color: getAgentColor(thread.parentMessage.createdBy) }}>
                                {thread.parentMessage.createdBy}
                              </span>
                              <span className="message-time">{formatTime(thread.parentMessage.timestamp)}</span>
                            </div>
                            {thread.isOrphaned && (
                              <div className="orphan-notice">Original message unavailable</div>
                            )}
                            <div className="message-content">
                              {thread.parentMessage.content}
                            </div>

                            {/* Reactions */}
                            {msgReactions.length > 0 && (
                              <div className="message-reactions">
                                {msgReactions.map((summary) => {
                                  const userReacted = summary.agentIds.includes('user')
                                  return (
                                    <ReactionTooltip
                                      key={summary.emoji}
                                      emoji={summary.emoji}
                                      names={formatReactionNames(summary)}
                                    >
                                      <button
                                        className={`reaction-badge clickable ${userReacted ? 'user-reacted' : ''}`}
                                        onClick={() => isWorkflowActive && handleReact(thread.parentMessage.id, summary.emoji, userReacted)}
                                        disabled={!isWorkflowActive}
                                      >
                                        <span className="reaction-emoji">{summary.emoji}</span>
                                        <span className="reaction-count">{summary.count}</span>
                                      </button>
                                    </ReactionTooltip>
                                  )
                                })}
                              </div>
                            )}

                            <button
                              className={`reply-indicator ${thread.replies.length === 0 ? 'empty' : ''}`}
                              onClick={() => openThread(thread)}
                              aria-label={`Comment on message from ${thread.parentMessage.createdBy}`}
                            >
                              {thread.replies.length > 0 ? (
                                <>
                                  <div className="reply-avatars">
                                    {getReplyAvatars(thread.replies).map((author, i) => (
                                      <div
                                        key={i}
                                        className="reply-avatar"
                                        style={{ background: getAgentColor(author) }}
                                      >
                                        {author.charAt(0).toUpperCase()}
                                      </div>
                                    ))}
                                  </div>
                                  <span className="reply-count">
                                    {thread.replies.length} {thread.replies.length === 1 ? 'reply' : 'replies'}
                                  </span>
                                  <span className="reply-preview">
                                    Last reply {formatDate(thread.replies[thread.replies.length - 1].timestamp)} at {formatTime(thread.replies[thread.replies.length - 1].timestamp)}
                                  </span>
                                </>
                              ) : (
                                <>
                                  <span className="reply-count">Add comment</span>
                                  <span className="reply-preview">Open thread to reply</span>
                                </>
                              )}
                            </button>
                          </div>
                        </div>
                        )
                      })
                    )}
                  </div>

                  <div className="channel-input-container">
                    <ChatInput
                      channelSlug={selectedChannel.slug}
                      placeholder={`Message #${selectedChannel.title}...`}
                      onSend={handleChannelSend}
                      disabled={!canCompose}
                      disabledReason="Unable to add comments to this session"
                      agentIds={['here', ...agents.map(a => a.id)]}
                    />
                  </div>
                </>
              )}
            </>
          ) : (
            <div className="no-channel-selected">
              <p>Select a channel to view messages</p>
            </div>
          )
        ) : (
          <div className="events-list">
            {events.map((event, idx) => {
              const author = event.event.thread?.created_by || event.event.agent_id
              const threadId = event.event.thread?.id
              // Don't show mentions for acked events - those are just message IDs
              const mentions = event.event.type === 'acked' 
                ? [] 
                : (event.event.mentions || event.event.thread?.mentions || [])
              return (
                <div key={idx} className="event-item">
                  <button 
                    className="event-header"
                    onClick={() => {
                      const newExpanded = new Set(expandedEvents)
                      if (newExpanded.has(idx)) {
                        newExpanded.delete(idx)
                      } else {
                        newExpanded.add(idx)
                      }
                      setExpandedEvents(newExpanded)
                    }}
                  >
                    <span className="event-expand">{expandedEvents.has(idx) ? '▼' : '▶'}</span>
                    <span className="event-type" style={{ color: getEventColor(event.event.type) }}>
                      {event.event.type}
                    </span>
                    {threadId && (
                      <span className="event-thread-id">{threadId}</span>
                    )}
                    {author && (
                      <span className="event-author" style={{ color: getAgentColor(author) }}>
                        {author}
                      </span>
                    )}
                    {mentions.length > 0 && (
                      <span className="event-mentions">
                        {mentions.map((m, i) => (
                          <span key={i} className="event-mention">@{m}</span>
                        ))}
                      </span>
                    )}
                    <span className="event-time">{formatTime(event.timestamp)}</span>
                  </button>
                  {expandedEvents.has(idx) && (
                    <pre className="event-detail">
                      {JSON.stringify(event.event, null, 2)}
                    </pre>
                  )}
                </div>
              )
            })}
          </div>
        )}
      </main>

      {/* Thread Panel */}
      {selectedThread && (
        <aside className="thread-panel">
          <header className="thread-header">
            <h3>Thread</h3>
            <button className="close-thread" onClick={() => setSelectedThread(null)}>
              ✕
            </button>
          </header>

          <div className="thread-content">
            {/* Parent message */}
            {(() => {
              const parentReactions = getReactionSummaries(selectedThread.parentMessage.id)
              return (
                <div className="thread-message parent">
                  {/* Hover reaction bar */}
                  {isWorkflowActive && (
                    <ReactionBar
                      messageId={selectedThread.parentMessage.id}
                      reactions={parentReactions}
                      onReact={(emoji, remove) => handleReact(selectedThread.parentMessage.id, emoji, remove)}
                    />
                  )}

                  <div className="message-avatar" style={{ background: getAgentColor(selectedThread.parentMessage.createdBy) }}>
                    {selectedThread.parentMessage.createdBy.charAt(0).toUpperCase()}
                  </div>
                  <div className="message-body">
                    <div className="message-header">
                      <span className="message-author" style={{ color: getAgentColor(selectedThread.parentMessage.createdBy) }}>
                        {selectedThread.parentMessage.createdBy}
                      </span>
                      <span className="message-time">{formatTime(selectedThread.parentMessage.timestamp)}</span>
                    </div>
                    {selectedThread.isOrphaned && (
                      <div className="orphan-notice">Original message unavailable</div>
                    )}
                    <div className="message-content">
                      {selectedThread.parentMessage.content}
                    </div>
                    {/* Parent message reactions */}
                    {parentReactions.length > 0 && (
                      <div className="message-reactions">
                        {parentReactions.map((summary) => {
                          const userReacted = summary.agentIds.includes('user')
                          return (
                            <ReactionTooltip
                              key={summary.emoji}
                              emoji={summary.emoji}
                              names={formatReactionNames(summary)}
                            >
                              <button
                                className={`reaction-badge clickable ${userReacted ? 'user-reacted' : ''}`}
                                onClick={() => isWorkflowActive && handleReact(selectedThread.parentMessage.id, summary.emoji, userReacted)}
                                disabled={!isWorkflowActive}
                              >
                                <span className="reaction-emoji">{summary.emoji}</span>
                                <span className="reaction-count">{summary.count}</span>
                              </button>
                            </ReactionTooltip>
                          )
                        })}
                      </div>
                    )}
                  </div>
                </div>
              )
            })()}

            {/* Reply count divider */}
            <div className="thread-divider">
              <span>{selectedThread.replies.length} {selectedThread.replies.length === 1 ? 'reply' : 'replies'}</span>
            </div>

            {/* Replies */}
            {selectedThread.replies.map((reply) => {
              const replyReactions = getReactionSummaries(reply.id)
              return (
                <div key={reply.id} className="thread-message reply">
                  {/* Hover reaction bar */}
                  {isWorkflowActive && (
                    <ReactionBar
                      messageId={reply.id}
                      reactions={replyReactions}
                      onReact={(emoji, remove) => handleReact(reply.id, emoji, remove)}
                    />
                  )}

                  <div className="message-avatar" style={{ background: getAgentColor(reply.createdBy) }}>
                    {reply.createdBy.charAt(0).toUpperCase()}
                  </div>
                  <div className="message-body">
                    <div className="message-header">
                      <span className="message-author" style={{ color: getAgentColor(reply.createdBy) }}>
                        {reply.createdBy}
                      </span>
                      <span className="message-time">{formatTime(reply.timestamp)}</span>
                    </div>
                    <div className="message-content">
                      {reply.content}
                    </div>
                    {reply.mentions.length > 0 && (
                      <div className="message-mentions">
                        {reply.mentions.map((m, i) => (
                          <span key={i} className="mention">@{m}</span>
                        ))}
                      </div>
                    )}
                    {/* Reply reactions */}
                    {replyReactions.length > 0 && (
                      <div className="message-reactions">
                        {replyReactions.map((summary) => {
                          const userReacted = summary.agentIds.includes('user')
                          return (
                            <ReactionTooltip
                              key={summary.emoji}
                              emoji={summary.emoji}
                              names={formatReactionNames(summary)}
                            >
                              <button
                                className={`reaction-badge clickable ${userReacted ? 'user-reacted' : ''}`}
                                onClick={() => isWorkflowActive && handleReact(reply.id, summary.emoji, userReacted)}
                                disabled={!isWorkflowActive}
                              >
                                <span className="reaction-emoji">{summary.emoji}</span>
                                <span className="reaction-count">{summary.count}</span>
                              </button>
                            </ReactionTooltip>
                          )
                        })}
                      </div>
                    )}
                  </div>
                </div>
              )
            })}
          </div>

          <div className="thread-input-container">
            <ChatInput
              channelSlug={selectedChannel?.slug || ''}
              threadId={selectedThread.parentMessage.id}
              placeholder="Reply to thread..."
              onSend={handleThreadReply}
              disabled={!canCompose}
              disabledReason="Unable to add comments to this session"
              agentIds={['here', ...agents.map(a => a.id)]}
            />
          </div>
        </aside>
      )}

      {/* Toast notification */}
      {toast && (
        <div className="fabric-panel-toast-container">
          <Toast
            message={toast.message}
            type={toast.type}
            duration={5000}
            onDismiss={() => setToast(null)}
          />
        </div>
      )}
    </div>
  )
}
