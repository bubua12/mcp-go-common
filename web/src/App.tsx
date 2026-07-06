import { useState, useEffect } from "react"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { TooltipProvider } from "@/components/ui/tooltip"
import { ClientConfigs } from "@/components/tabs/client-configs"
import { MCPInspector } from "@/components/tabs/mcp-inspector"
import { Settings, Wrench, ExternalLink } from "lucide-react"
import { Button } from "@/components/ui/button"

interface ServerInfo {
  name: string
  version: string
}

export default function App() {
  const [serverInfo, setServerInfo] = useState<ServerInfo>({ name: "MCP Server", version: "..." })

  useEffect(() => {
    fetch("/api/info")
      .then(r => r.json())
      .then(data => setServerInfo({ name: data.name, version: data.version }))
      .catch(() => {})
  }, [])

  return (
    <TooltipProvider>
      <div className="min-h-screen bg-background">
        <header className="border-b bg-background sticky top-0 z-50">
          <div className="container mx-auto px-4 h-14 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground font-bold text-sm">
                {serverInfo.name.charAt(0).toUpperCase()}
              </div>
              <div className="flex flex-col">
                <span className="font-semibold text-lg leading-tight">{serverInfo.name}</span>
                <span className="text-xs text-muted-foreground leading-tight">v{serverInfo.version}</span>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Button variant="ghost" size="sm" asChild>
                <a
                  href="https://github.com/bubua12/mcp-go-common"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center gap-1.5"
                >
                  Documentation
                  <ExternalLink className="h-3 w-3" />
                </a>
              </Button>
            </div>
          </div>
        </header>

        <main className="container mx-auto px-4 py-6">
          <Tabs defaultValue="setup" className="space-y-6">
            <TabsList className="grid w-full max-w-md grid-cols-2">
              <TabsTrigger value="setup" className="flex items-center gap-2">
                <Settings className="h-4 w-4" />
                Setup
              </TabsTrigger>
              <TabsTrigger value="inspector" className="flex items-center gap-2">
                <Wrench className="h-4 w-4" />
                Inspector
              </TabsTrigger>
            </TabsList>

            <TabsContent value="setup">
              <ClientConfigs serverName={serverInfo.name} />
            </TabsContent>

            <TabsContent value="inspector">
              <MCPInspector />
            </TabsContent>
          </Tabs>
        </main>
      </div>
    </TooltipProvider>
  )
}
