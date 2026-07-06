import * as React from "react"
import { useEffect, useState } from "react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import {
  Wrench,
  ChevronRight,
  ChevronDown,
  Play,
  Loader2,
  AlertCircle,
  CheckCircle,
  RefreshCw,
  Plug,
  PlugZap,
} from "lucide-react"
import { useMCPClient, MCPTool } from "@/hooks/use-mcp-client"

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
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Executing...
                </>
              ) : (
                <>
                  <Play className="h-4 w-4" />
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

export function MCPInspector() {
  const {
    serverInfo,
    tools,
    isConnected,
    isConnecting,
    error,
    connect,
    disconnect,
    callTool,
  } = useMCPClient()

  const [isExecuting, setIsExecuting] = useState(false)
  const [searchQuery, setSearchQuery] = useState("")

  const handleCallTool = async (name: string, args: Record<string, unknown>) => {
    setIsExecuting(true)
    try {
      return await callTool(name, args)
    } finally {
      setIsExecuting(false)
    }
  }

  const filteredTools = tools.filter(
    t => t.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
         t.description?.toLowerCase().includes(searchQuery.toLowerCase())
  )

  useEffect(() => {
    if (isConnected || isConnecting) return
    connect().catch(() => {})
  }, [])

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <h2 className="text-2xl font-bold tracking-tight">MCP Inspector</h2>
        <p className="text-muted-foreground">
          Connect to the MCP server to inspect and test available tools.
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
                    <RefreshCw className="h-4 w-4" />
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
                      <Loader2 className="h-4 w-4 animate-spin" />
                      Connecting...
                    </>
                  ) : (
                    <>
                      <Plug className="h-4 w-4" />
                      Connect
                    </>
                  )}
                </Button>
              )}
            </div>
          </div>
        </CardHeader>
        {isConnected && serverInfo && (
          <CardContent className="pt-0">
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
              <div>
                <span className="text-muted-foreground">Tools</span>
                <p className="font-medium">{tools.length}</p>
              </div>
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
          <Input
            placeholder="Search tools..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="max-w-md"
          />

          {filteredTools.length > 0 && (
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
        </>
      )}
    </div>
  )
}
