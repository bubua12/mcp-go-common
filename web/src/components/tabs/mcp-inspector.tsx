import * as React from "react"
import { useEffect, useState } from "react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import {
  Wrench,
  FileText,
  MessageSquareQuote,
  Sparkles,
  ChevronRight,
  ChevronDown,
  Play,
  Eye,
  Loader2,
  AlertCircle,
  CheckCircle,
  RefreshCw,
  Plug,
  PlugZap,
} from "lucide-react"
import { useMCPClient, MCPTool, MCPResource, MCPPrompt } from "@/hooks/use-mcp-client"

interface ToolItemProps {
  tool: MCPTool
  onExecute: (name: string, args: Record<string, unknown>) => Promise<unknown>
  isExecuting: boolean
}

function ToolItem({ tool, onExecute, isExecuting }: ToolItemProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [args, setArgs] = useState<Record<string, string>>({})
  const [result, setResult] = useState<unknown>(null)
  const [error, setError] = useState<string | null>(null)

  const schema = tool.inputSchema as { properties?: Record<string, { type: string; description?: string }>; required?: string[] } | undefined
  const properties = schema?.properties || {}
  const required = schema?.required || []

  const handleExecute = async () => {
    setError(null)
    setResult(null)
    try {
      const parsedArgs: Record<string, unknown> = {}
      for (const [key, value] of Object.entries(args)) {
        if (value) {
          try {
            parsedArgs[key] = JSON.parse(value)
          } catch {
            parsedArgs[key] = value
          }
        }
      }
      const res = await onExecute(tool.name, parsedArgs)
      setResult(res)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <Collapsible open={isOpen} onOpenChange={setIsOpen}>
      <Card>
        <CollapsibleTrigger asChild>
          <CardHeader className="cursor-pointer hover:bg-muted/50 transition-colors py-3">
            <div className="flex items-center gap-3">
              {isOpen ? (
                <ChevronDown className="h-4 w-4 text-muted-foreground" />
              ) : (
                <ChevronRight className="h-4 w-4 text-muted-foreground" />
              )}
              <Wrench className="h-4 w-4 text-primary" />
              <div className="flex-1">
                <CardTitle className="text-sm font-medium">{tool.name}</CardTitle>
                {tool.description && (
                  <CardDescription className="text-xs mt-0.5 line-clamp-1">
                    {tool.description}
                  </CardDescription>
                )}
              </div>
              <Badge variant="outline" className="text-xs">
                {Object.keys(properties).length} params
              </Badge>
            </div>
          </CardHeader>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <CardContent className="pt-0 space-y-4">
            {tool.description && (
              <p className="text-sm text-muted-foreground">{tool.description}</p>
            )}

            {Object.keys(properties).length > 0 && (
              <div className="space-y-3">
                <h4 className="text-sm font-medium">Parameters</h4>
                {Object.entries(properties).map(([name, prop]) => (
                  <div key={name} className="space-y-1">
                    <label className="text-sm font-medium flex items-center gap-2">
                      {name}
                      {required.includes(name) && (
                        <Badge variant="destructive" className="text-xs">required</Badge>
                      )}
                      <span className="text-xs text-muted-foreground">({prop.type})</span>
                    </label>
                    {prop.description && (
                      <p className="text-xs text-muted-foreground">{prop.description}</p>
                    )}
                    <Input
                      placeholder={`Enter ${name}...`}
                      value={args[name] || ""}
                      onChange={(e) => setArgs({ ...args, [name]: e.target.value })}
                      className="font-mono text-sm"
                    />
                  </div>
                ))}
              </div>
            )}

            <Button onClick={handleExecute} disabled={isExecuting} className="w-full">
              {isExecuting ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin mr-2" />
                  Executing...
                </>
              ) : (
                <>
                  <Play className="h-4 w-4 mr-2" />
                  Execute Tool
                </>
              )}
            </Button>

            {error && (
              <div className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive flex items-start gap-2">
                <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" />
                <span>{error}</span>
              </div>
            )}

            {result && (
              <div className="space-y-2">
                <div className="flex items-center gap-2 text-sm font-medium text-green-600">
                  <CheckCircle className="h-4 w-4" />
                  Result
                </div>
                <pre className="overflow-x-auto rounded-lg bg-slate-900 p-4 text-sm text-slate-100 max-h-64">
                  <code>{JSON.stringify(result, null, 2)}</code>
                </pre>
              </div>
            )}
          </CardContent>
        </CollapsibleContent>
      </Card>
    </Collapsible>
  )
}

interface ResourceItemProps {
  resource: MCPResource
  onRead: (uri: string) => Promise<unknown>
  isReading: boolean
}

function ResourceItem({ resource, onRead, isReading }: ResourceItemProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [result, setResult] = useState<unknown>(null)
  const [error, setError] = useState<string | null>(null)

  const handleRead = async () => {
    setError(null)
    setResult(null)
    try {
      const res = await onRead(resource.uri)
      setResult(res)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <Collapsible open={isOpen} onOpenChange={setIsOpen}>
      <Card>
        <CollapsibleTrigger asChild>
          <CardHeader className="cursor-pointer hover:bg-muted/50 transition-colors py-3">
            <div className="flex items-center gap-3">
              {isOpen ? (
                <ChevronDown className="h-4 w-4 text-muted-foreground" />
              ) : (
                <ChevronRight className="h-4 w-4 text-muted-foreground" />
              )}
              <FileText className="h-4 w-4 text-blue-500" />
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <CardTitle className="text-sm font-medium truncate">
                    {resource.title || resource.name}
                  </CardTitle>
                </div>
                <CardDescription className="text-xs mt-0.5 font-mono truncate">
                  {resource.uri}
                </CardDescription>
              </div>
              {resource.mimeType && (
                <Badge variant="outline" className="text-xs">
                  {resource.mimeType}
                </Badge>
              )}
            </div>
          </CardHeader>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <CardContent className="pt-0 space-y-4">
            {resource.description && (
              <p className="text-sm text-muted-foreground">{resource.description}</p>
            )}

            <Button onClick={handleRead} disabled={isReading} variant="secondary" className="w-full">
              {isReading ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin mr-2" />
                  Reading Resource...
                </>
              ) : (
                <>
                  <Eye className="h-4 w-4 mr-2" />
                  Read Resource
                </>
              )}
            </Button>

            {error && (
              <div className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive flex items-start gap-2">
                <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" />
                <span>{error}</span>
              </div>
            )}

            {result && (
              <div className="space-y-2">
                <div className="flex items-center gap-2 text-sm font-medium text-green-600">
                  <CheckCircle className="h-4 w-4" />
                  Content
                </div>
                <pre className="overflow-x-auto rounded-lg bg-slate-900 p-4 text-sm text-slate-100 max-h-96">
                  <code>{JSON.stringify(result, null, 2)}</code>
                </pre>
              </div>
            )}
          </CardContent>
        </CollapsibleContent>
      </Card>
    </Collapsible>
  )
}

interface PromptItemProps {
  prompt: MCPPrompt
  onGet: (name: string, args: Record<string, string>) => Promise<unknown>
  isGetting: boolean
}

function PromptItem({ prompt, onGet, isGetting }: PromptItemProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [args, setArgs] = useState<Record<string, string>>({})
  const [result, setResult] = useState<unknown>(null)
  const [error, setError] = useState<string | null>(null)

  const handleGet = async () => {
    setError(null)
    setResult(null)
    try {
      const res = await onGet(prompt.name, args)
      setResult(res)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  const argList = prompt.arguments || []

  return (
    <Collapsible open={isOpen} onOpenChange={setIsOpen}>
      <Card>
        <CollapsibleTrigger asChild>
          <CardHeader className="cursor-pointer hover:bg-muted/50 transition-colors py-3">
            <div className="flex items-center gap-3">
              {isOpen ? (
                <ChevronDown className="h-4 w-4 text-muted-foreground" />
              ) : (
                <ChevronRight className="h-4 w-4 text-muted-foreground" />
              )}
              <MessageSquareQuote className="h-4 w-4 text-amber-500" />
              <div className="flex-1 min-w-0">
                <CardTitle className="text-sm font-medium">
                  {prompt.title || prompt.name}
                </CardTitle>
                {prompt.description && (
                  <CardDescription className="text-xs mt-0.5 line-clamp-1">
                    {prompt.description}
                  </CardDescription>
                )}
              </div>
              {argList.length > 0 && (
                <Badge variant="outline" className="text-xs">
                  {argList.length} args
                </Badge>
              )}
            </div>
          </CardHeader>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <CardContent className="pt-0 space-y-4">
            {prompt.description && (
              <p className="text-sm text-muted-foreground">{prompt.description}</p>
            )}

            {argList.length > 0 && (
              <div className="space-y-3">
                <h4 className="text-sm font-medium">Arguments</h4>
                {argList.map((arg) => (
                  <div key={arg.name} className="space-y-1">
                    <label className="text-sm font-medium flex items-center gap-2">
                      {arg.name}
                      {arg.required && (
                        <Badge variant="destructive" className="text-xs">required</Badge>
                      )}
                    </label>
                    {arg.description && (
                      <p className="text-xs text-muted-foreground">{arg.description}</p>
                    )}
                    <Input
                      placeholder={`Enter ${arg.name}...`}
                      value={args[arg.name] || ""}
                      onChange={(e) => setArgs({ ...args, [arg.name]: e.target.value })}
                      className="font-mono text-sm"
                    />
                  </div>
                ))}
              </div>
            )}

            <Button onClick={handleGet} disabled={isGetting} variant="secondary" className="w-full">
              {isGetting ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin mr-2" />
                  Generating Prompt...
                </>
              ) : (
                <>
                  <Sparkles className="h-4 w-4 mr-2" />
                  Get Prompt
                </>
              )}
            </Button>

            {error && (
              <div className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive flex items-start gap-2">
                <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" />
                <span>{error}</span>
              </div>
            )}

            {result && (
              <div className="space-y-2">
                <div className="flex items-center gap-2 text-sm font-medium text-green-600">
                  <CheckCircle className="h-4 w-4" />
                  Prompt Result
                </div>
                <pre className="overflow-x-auto rounded-lg bg-slate-900 p-4 text-sm text-slate-100 max-h-96">
                  <code>{JSON.stringify(result, null, 2)}</code>
                </pre>
              </div>
            )}
          </CardContent>
        </CollapsibleContent>
      </Card>
    </Collapsible>
  )
}

export function MCPInspector() {
  const {
    serverInfo,
    tools,
    resources,
    prompts,
    isConnected,
    isConnecting,
    error,
    connect,
    disconnect,
    callTool,
    readResource,
    getPrompt,
  } = useMCPClient()

  const [isExecuting, setIsExecuting] = useState(false)
  const [isReading, setIsReading] = useState(false)
  const [isGetting, setIsGetting] = useState(false)
  const [searchQuery, setSearchQuery] = useState("")

  const handleCallTool = async (name: string, args: Record<string, unknown>) => {
    setIsExecuting(true)
    try {
      return await callTool(name, args)
    } finally {
      setIsExecuting(false)
    }
  }

  const handleReadResource = async (uri: string) => {
    setIsReading(true)
    try {
      return await readResource(uri)
    } finally {
      setIsReading(false)
    }
  }

  const handleGetPrompt = async (name: string, args: Record<string, string>) => {
    setIsGetting(true)
    try {
      return await getPrompt(name, args)
    } finally {
      setIsGetting(false)
    }
  }

  const q = searchQuery.toLowerCase().trim()

  const filteredTools = tools.filter(
    t => !q || t.name.toLowerCase().includes(q) || t.description?.toLowerCase().includes(q)
  )

  const filteredResources = resources.filter(
    r => !q ||
      r.name.toLowerCase().includes(q) ||
      r.uri.toLowerCase().includes(q) ||
      r.title?.toLowerCase().includes(q) ||
      r.description?.toLowerCase().includes(q)
  )

  const filteredPrompts = prompts.filter(
    p => !q ||
      p.name.toLowerCase().includes(q) ||
      p.title?.toLowerCase().includes(q) ||
      p.description?.toLowerCase().includes(q)
  )

  // 动态生成搜索框占位符
  const availableCategories: string[] = []
  if (tools.length > 0) availableCategories.push("tools")
  if (resources.length > 0) availableCategories.push("resources")
  if (prompts.length > 0) availableCategories.push("prompts")
  const searchPlaceholder = availableCategories.length > 0
    ? `Search ${availableCategories.join(", ")}...`
    : "Search..."

  const hasAnyItems = tools.length > 0 || resources.length > 0 || prompts.length > 0
  const hasMatchingItems = filteredTools.length > 0 || filteredResources.length > 0 || filteredPrompts.length > 0

  useEffect(() => {
    if (isConnected || isConnecting) return
    connect().catch(() => {})
  }, [])

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <h2 className="text-2xl font-bold tracking-tight">MCP Inspector</h2>
        <p className="text-muted-foreground">
          Connect to the MCP server to inspect and test available capabilities.
        </p>
      </div>

      {/* Connection Status */}
      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className={`flex h-10 w-10 items-center justify-center rounded-lg ${isConnected ? 'bg-green-100' : 'bg-muted'}`}>
                {isConnected ? (
                  <PlugZap className="h-5 w-5 text-green-600" />
                ) : (
                  <Plug className="h-5 w-5 text-muted-foreground" />
                )}
              </div>
              <div>
                <CardTitle className="text-base">
                  {isConnected ? 'Connected' : 'Disconnected'}
                </CardTitle>
                <CardDescription>
                  {isConnected && serverInfo
                    ? `${serverInfo.name} v${serverInfo.version}`
                    : 'Click connect to start inspecting'
                  }
                </CardDescription>
              </div>
            </div>
            <div className="flex gap-2">
              {isConnected ? (
                <>
                  <Button variant="outline" size="sm" onClick={connect}>
                    <RefreshCw className="h-4 w-4 mr-2" />
                    Refresh
                  </Button>
                  <Button variant="destructive" size="sm" onClick={disconnect}>
                    Disconnect
                  </Button>
                </>
              ) : (
                <Button onClick={connect} disabled={isConnecting}>
                  {isConnecting ? (
                    <>
                      <Loader2 className="h-4 w-4 animate-spin mr-2" />
                      Connecting...
                    </>
                  ) : (
                    <>
                      <Plug className="h-4 w-4 mr-2" />
                      Connect
                    </>
                  )}
                </Button>
              )}
            </div>
          </div>
        </CardHeader>
        {isConnected && serverInfo && hasAnyItems && (
          <CardContent className="pt-0">
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
              {tools.length > 0 && (
                <div>
                  <span className="text-muted-foreground">Tools</span>
                  <p className="font-medium">{tools.length}</p>
                </div>
              )}
              {resources.length > 0 && (
                <div>
                  <span className="text-muted-foreground">Resources</span>
                  <p className="font-medium">{resources.length}</p>
                </div>
              )}
              {prompts.length > 0 && (
                <div>
                  <span className="text-muted-foreground">Prompts</span>
                  <p className="font-medium">{prompts.length}</p>
                </div>
              )}
            </div>
          </CardContent>
        )}
        {error && (
          <CardContent className="pt-0">
            <div className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive flex items-start gap-2">
              <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" />
              <span>{error.message}</span>
            </div>
          </CardContent>
        )}
      </Card>

      {isConnected && (
        <>
          {hasAnyItems && (
            <Input
              placeholder={searchPlaceholder}
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="max-w-md"
            />
          )}

          {/* Tools 区域：只有 tools 存在且匹配时展示 */}
          {tools.length > 0 && filteredTools.length > 0 && (
            <div className="space-y-3">
              <div className="flex items-center gap-2">
                <Wrench className="h-5 w-5 text-primary" />
                <h3 className="text-lg font-semibold">Tools ({filteredTools.length})</h3>
              </div>
              <div className="space-y-2">
                {filteredTools.map((tool) => (
                  <ToolItem
                    key={tool.name}
                    tool={tool}
                    onExecute={handleCallTool}
                    isExecuting={isExecuting}
                  />
                ))}
              </div>
            </div>
          )}

          {/* Resources 区域：只有 resources 存在且匹配时才展示；不存在时整个标题都不渲染 */}
          {resources.length > 0 && filteredResources.length > 0 && (
            <div className="space-y-3">
              <div className="flex items-center gap-2">
                <FileText className="h-5 w-5 text-blue-500" />
                <h3 className="text-lg font-semibold">Resources ({filteredResources.length})</h3>
              </div>
              <div className="space-y-2">
                {filteredResources.map((resource) => (
                  <ResourceItem
                    key={resource.uri}
                    resource={resource}
                    onRead={handleReadResource}
                    isReading={isReading}
                  />
                ))}
              </div>
            </div>
          )}

          {/* Prompts 区域：只有 prompts 存在且匹配时才展示；不存在时整个标题都不渲染 */}
          {prompts.length > 0 && filteredPrompts.length > 0 && (
            <div className="space-y-3">
              <div className="flex items-center gap-2">
                <MessageSquareQuote className="h-5 w-5 text-amber-500" />
                <h3 className="text-lg font-semibold">Prompts ({filteredPrompts.length})</h3>
              </div>
              <div className="space-y-2">
                {filteredPrompts.map((prompt) => (
                  <PromptItem
                    key={prompt.name}
                    prompt={prompt}
                    onGet={handleGetPrompt}
                    isGetting={isGetting}
                  />
                ))}
              </div>
            </div>
          )}

          {/* 搜索无结果提示 */}
          {hasAnyItems && !hasMatchingItems && (
            <div className="py-8 text-center text-sm text-muted-foreground">
              未找到与 &quot;{searchQuery}&quot; 匹配的 Tools、Resources 或 Prompts
            </div>
          )}
        </>
      )}
    </div>
  )
}
