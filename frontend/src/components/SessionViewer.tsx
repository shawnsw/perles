import { useState, useEffect } from 'react'
import type { Session } from '../types'
import MetadataPanel from './MetadataPanel'
import FabricPanel from './FabricPanel'
import AgentPanel from './AgentPanel'
import McpPanel from './McpPanel'
import CommandsPanel from './CommandsPanel'
import ObserverPanel from './ObserverPanel'
import './SessionViewer.css'

interface Props {
  session: Session
}

type Tab = 'overview' | 'fabric' | 'commands' | 'coordinator' | 'workers' | 'observer' | 'mcp'

const validTabs: Tab[] = ['overview', 'fabric', 'commands', 'coordinator', 'workers', 'observer', 'mcp']

function getInitialTab(): Tab {
  const params = new URLSearchParams(window.location.search)
  const tabFromUrl = params.get('tab')
  if (tabFromUrl && validTabs.includes(tabFromUrl as Tab)) {
    return tabFromUrl as Tab
  }
  return 'overview'
}

export default function SessionViewer({ session }: Props) {
  const [activeTab, setActiveTab] = useState<Tab>(getInitialTab)
  const [selectedWorker, setSelectedWorker] = useState<string>(
    Object.keys(session.workers)[0] || ''
  )
  const [mcpWorkerFilter, setMcpWorkerFilter] = useState<string | undefined>(undefined)

  // Update URL when tab changes
  const handleTabChange = (tab: Tab) => {
    setActiveTab(tab)
    const url = new URL(window.location.href)
    url.searchParams.set('tab', tab)
    window.history.pushState({ tab }, '', url.toString())
  }

  // Listen for browser back/forward navigation
  useEffect(() => {
    const handlePopState = () => {
      const tab = getInitialTab()
      setActiveTab(tab)
      setMcpWorkerFilter(undefined) // Reset filter on navigation
    }
    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [])

  const tabs: { id: Tab; label: string; count?: number; hidden?: boolean }[] = [
    { id: 'overview', label: 'Overview' },
    { id: 'fabric', label: 'Fabric', count: session.fabric.length },
    { id: 'commands', label: 'Commands', count: session.commands?.length || 0 },
    { id: 'coordinator', label: 'Coordinator', count: session.coordinator.messages.length },
    { id: 'workers', label: 'Workers', count: Object.keys(session.workers).length },
    { id: 'observer', label: 'Observer', count: session.observer?.messages.length || 0, hidden: !session.observer },
    { id: 'mcp', label: 'MCP Requests', count: session.mcpRequests.length },
  ]

  return (
    <div className="session-viewer">
      <nav className="viewer-tabs">
        {tabs.filter(tab => !tab.hidden).map(tab => (
          <button
            key={tab.id}
            className={`tab-btn ${activeTab === tab.id ? 'active' : ''}`}
            onClick={() => handleTabChange(tab.id)}
          >
            {tab.label}
            {tab.count !== undefined && (
              <span className="tab-count">{tab.count}</span>
            )}
          </button>
        ))}
      </nav>

      <div className="viewer-content">
        {activeTab === 'overview' && session.metadata && (
          <MetadataPanel 
            metadata={session.metadata} 
            session={session}
            onNavigateToWorker={(workerId) => {
              setSelectedWorker(workerId)
              handleTabChange('workers')
            }}
            onNavigateToMcp={(workerId) => {
              setMcpWorkerFilter(workerId)
              handleTabChange('mcp')
            }}
          />
        )}
        
        {activeTab === 'fabric' && (
          <FabricPanel
            events={session.fabric}
            workflowId={session.metadata?.workflow_id}
            sessionPath={session.path}
          />
        )}

        {activeTab === 'commands' && (
          <CommandsPanel commands={session.commands || []} />
        )}
        
        {activeTab === 'coordinator' && (
          <AgentPanel 
            name="Coordinator" 
            messages={session.coordinator.messages} 
          />
        )}
        
        {activeTab === 'workers' && (
          <div className="workers-view">
            <div className="worker-selector">
              {Object.keys(session.workers).map(workerId => (
                <button
                  key={workerId}
                  className={`worker-btn ${selectedWorker === workerId ? 'active' : ''}`}
                  onClick={() => setSelectedWorker(workerId)}
                >
                  {workerId}
                  <span className="msg-count">
                    {session.workers[workerId].messages.length} msgs
                  </span>
                </button>
              ))}
            </div>
            {selectedWorker && session.workers[selectedWorker] && (
              <AgentPanel 
                name={selectedWorker} 
                messages={session.workers[selectedWorker].messages}
                hideHeader
              />
            )}
          </div>
        )}
        
        {activeTab === 'observer' && session.observer && (
          <ObserverPanel 
            messages={session.observer.messages} 
            notes={session.observer.notes}
          />
        )}

        {activeTab === 'mcp' && (
          <McpPanel requests={session.mcpRequests} initialWorkerFilter={mcpWorkerFilter} />
        )}
      </div>
    </div>
  )
}
