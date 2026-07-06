import { useState, useEffect } from "react"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { TooltipProvider } from "@/components/ui/tooltip"
import { ClientConfigs } from "@/components/tabs/client-configs"
import { MCPInspector } from "@/components/tabs/mcp-inspector"
import { Settings, Wrench, ExternalLink } from "lucide-react"
import { Button } from "@/components/ui/button"

interface ServerInfo {
  name: string
  description: string
}

export default function App() {
  const [serverInfo, setServerInfo] = useState<ServerInfo>({ name: "MCP Server", description: "" })

  useEffect(() => {
    fetch("/api/info")
      .then(r => r.json())
      .then(data => setServerInfo({ name: data.name, description: data.description }))
      .catch(() => {})
  }, [])

  return (
    <TooltipProvider>
      <div className="min-h-screen bg-background">
        <header className="border-b bg-background sticky top-0 z-50">
          <div className="container mx-auto px-4 h-14 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <svg xmlns="http://www.w3.org/2000/svg" className="h-8 w-8 text-foreground" viewBox="0 0 24 24" fill="currentColor">
                <path d="M13.85 0a4.16 4.16 0 0 0-2.95 1.217L1.456 10.66a.835.835 0 0 0 0 1.18a.835.835 0 0 0 1.18 0l9.442-9.442a2.49 2.49 0 0 1 3.541 0a2.49 2.49 0 0 1 0 3.541L8.59 12.97l-.1.1a.835.835 0 0 0 0 1.18a.835.835 0 0 0 1.18 0l.1-.098l7.03-7.034a2.49 2.49 0 0 1 3.542 0l.049.05a2.49 2.49 0 0 1 0 3.54l-8.54 8.54a1.96 1.96 0 0 0 0 2.755l1.753 1.753a.835.835 0 0 0 1.18 0a.835.835 0 0 0 0-1.18l-1.753-1.753a.266.266 0 0 1 0-.394l8.54-8.54a4.185 4.185 0 0 0 0-5.9l-.05-.05a4.16 4.16 0 0 0-2.95-1.218c-.2 0-.401.02-.6.048a4.17 4.17 0 0 0-1.17-3.552A4.16 4.16 0 0 0 13.85 0m0 3.333a.84.84 0 0 0-.59.245L6.275 10.56a4.186 4.186 0 0 0 0 5.902a4.186 4.186 0 0 0 5.902 0L19.16 9.48a.835.835 0 0 0 0-1.18a.835.835 0 0 0-1.18 0l-6.985 6.984a2.49 2.49 0 0 1-3.54 0a2.49 2.49 0 0 1 0-3.54l6.983-6.985a.835.835 0 0 0 0-1.18a.84.84 0 0 0-.59-.245"/>
              </svg>
              <div className="flex flex-col">
                <span className="font-semibold text-lg leading-tight">{serverInfo.name}</span>
                {serverInfo.description && (
                  <span className="text-xs text-muted-foreground leading-tight">{serverInfo.description}</span>
                )}
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
