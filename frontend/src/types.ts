export interface TokenUsage {
  context_tokens?: number;
  total_input_tokens?: number;  // Legacy field name
  total_output_tokens: number;
  total_cost_usd: number;
}

export interface SessionMetadata {
  session_id: string;
  start_time: string;
  status: string;
  session_dir: string;
  coordinator_session_ref: string;
  resumable: boolean;
  workers: WorkerMeta[];
  client_type: string;
  token_usage: TokenUsage;
  coordinator_token_usage?: TokenUsage;
  application_name: string;
  work_dir: string;
  date_partition: string;
  workflow_id?: string;
}

export interface WorkerMeta {
  id: string;
  spawned_at: string;
  headless_session_ref: string;
  work_dir: string;
  token_usage?: TokenUsage;
}

export interface FabricEvent {
  version: number;
  timestamp: string;
  event: {
    type: string;
    timestamp: string;
    channel_id?: string;
    channel_slug?: string;
    parent_id?: string;
    agent_id?: string;
    thread?: {
      id: string;
      type: string;
      created_at: string;
      created_by: string;
      content?: string;
      kind?: string;
      slug?: string;
      title?: string;
      purpose?: string;
      mentions?: string[];
      seq: number;
      // Artifact fields
      name?: string;
      media_type?: string;
      size_bytes?: number;
      storage_uri?: string;
      sha256?: string;
    };
    subscription?: {
      channel_id: string;
      agent_id: string;
      mode: string;
    };
    reaction?: {
      thread_id: string;
      agent_id: string;
      emoji: string;
      created_at: string;
    };
    mentions?: string[];
  };
}

export interface McpRequest {
  timestamp: string;
  type: string;
  method: string;
  tool_name: string;
  request_json: string;
  response_json: string;
  duration: number;
  worker_id?: string;
}

export interface AgentMessage {
  role: string;
  content: string;
  is_tool_call?: boolean;
  ts: string;
}

export interface Command {
  command_id: string;
  command_type: string;
  source: string;
  success: boolean;
  error?: string;
  duration_ms: number;
  timestamp: string;
  payload: Record<string, unknown>;
  result_data?: Record<string, unknown>;
}

export interface Agent {
  id: string;
  role: string;
}

export interface AgentsResponse {
  agents: Agent[];
  isActive: boolean;
}

export interface ObserverData {
  messages: AgentMessage[];
  notes: string;
}

export interface Session {
  path: string;
  metadata: SessionMetadata | null;
  fabric: FabricEvent[];
  mcpRequests: McpRequest[];
  commands: Command[];
  messages: unknown[];
  coordinator: {
    messages: AgentMessage[];
    raw: unknown[];
  };
  workers: {
    [key: string]: {
      messages: AgentMessage[];
      raw: unknown[];
      accountabilitySummary?: string;
    };
  };
  observer?: ObserverData;
  accountabilitySummary: string | null;
}
