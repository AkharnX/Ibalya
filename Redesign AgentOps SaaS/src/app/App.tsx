import React, { useState, useEffect } from "react";
import {
  Inbox,
  AlertTriangle,
  CheckCircle2,
  Settings,
  Search,
  Filter,
  Mail,
  ShieldCheck,
  Send,
  X,
  User,
  Moon,
  Sun,
  ArrowRight,
  Command,
  FileText,
  ScanText
} from "lucide-react";
import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";
import * as Dialog from "@radix-ui/react-dialog";

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

// --- Data Models ---
type Status = "risk" | "delay" | "healthy";

interface Engagement {
  id: string;
  type: string;
  title: string;
  deadline: string;
  reliability: number;
  status: Status;
  client: string;
  blocker?: string;
  sourceEmail: string;
}

const mockEngagements: Engagement[] = [
  {
    id: "ENG-102",
    type: "Livrable",
    title: "Maquette v2",
    deadline: "24/10",
    reliability: 92,
    status: "delay",
    client: "Acme Corp",
    blocker: "Attente validation",
    sourceEmail: "Re: Suite point hebdo",
  },
  {
    id: "ENG-103",
    type: "Paiement",
    title: "Facture F-2023-11",
    deadline: "15/10",
    reliability: 98,
    status: "risk",
    client: "Globex",
    blocker: "Silence > 14j",
    sourceEmail: "Relance comptabilité",
  },
  {
    id: "ENG-104",
    type: "Contrat",
    title: "Signature NDA",
    deadline: "Auj.",
    reliability: 85,
    status: "healthy",
    client: "Stark Ind.",
    sourceEmail: "Fwd: NDA à signer",
  },
];

const kpis = {
  active: 142,
  delayed: 12,
  atRisk: 3,
  toValidate: 5,
};

// --- Components ---

function Badge({ status, children, className }: { status: Status; children: React.ReactNode; className?: string }) {
  const statusStyles = {
    risk: "bg-risk-bg text-risk border-risk/30",
    delay: "bg-delay-bg text-delay border-delay/30",
    healthy: "bg-healthy-bg text-healthy border-healthy/30",
  };
  return (
    <span className={cn("inline-flex items-center px-1.5 py-0.5 text-[10px] font-mono uppercase tracking-widest border", statusStyles[status], className)}>
      {children}
    </span>
  );
}

function Button({ children, variant = "default", size = "default", className, ...props }: any) {
  const base = "inline-flex items-center justify-center text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-primary disabled:pointer-events-none disabled:opacity-50";
  const variants = {
    default: "bg-primary text-primary-foreground hover:bg-primary/90 shadow-sm",
    outline: "border border-border bg-transparent hover:bg-secondary text-foreground",
    ghost: "hover:bg-secondary hover:text-foreground text-muted-foreground",
  };
  const sizes = {
    default: "h-8 px-4",
    sm: "h-7 px-3 text-xs",
    icon: "h-8 w-8",
  };
  return (
    <button className={cn(base, variants[variant as keyof typeof variants], sizes[size as keyof typeof sizes], className)} {...props}>
      {children}
    </button>
  );
}

// --- Main App ---

export default function App() {
  const [currentView, setCurrentView] = useState("dashboard");
  const [isDarkMode, setIsDarkMode] = useState(true);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [selectedEngagement, setSelectedEngagement] = useState<Engagement | null>(null);

  useEffect(() => {
    if (isDarkMode) {
      document.documentElement.classList.add("dark");
    } else {
      document.documentElement.classList.remove("dark");
    }
  }, [isDarkMode]);

  const openAction = (eng: Engagement) => {
    setSelectedEngagement(eng);
    setDrawerOpen(true);
  };

  const NavItem = ({ icon: Icon, label, id, count }: any) => (
    <button
      onClick={() => setCurrentView(id)}
      className={cn(
        "group flex items-center justify-between w-full px-3 py-2 text-sm transition-colors relative",
        currentView === id 
          ? "text-sidebar-foreground bg-primary/10" 
          : "text-sidebar-foreground/60 hover:text-sidebar-foreground hover:bg-sidebar-border/50"
      )}
    >
      {currentView === id && <div className="absolute left-0 top-0 bottom-0 w-0.5 bg-primary" />}
      <div className="flex items-center gap-3">
        <Icon className={cn("w-4 h-4", currentView === id ? "text-primary" : "")} />
        {label}
      </div>
      {count > 0 && (
        <span className={cn(
          "px-1.5 py-0.5 text-[10px] font-mono",
          currentView === id ? "bg-primary text-primary-foreground" : "bg-sidebar-border text-sidebar-foreground/60"
        )}>
          {count}
        </span>
      )}
    </button>
  );

  return (
    <div className="min-h-screen bg-background text-foreground font-sans flex overflow-hidden">
      
      {/* Sidebar - Identité visuelle forte : toujours sombre, très structurée */}
      <aside className="w-[260px] border-r border-sidebar-border bg-[var(--sidebar)] text-[var(--sidebar-foreground)] flex flex-col hidden md:flex shrink-0">
        <div className="h-14 flex items-center px-5 border-b border-sidebar-border">
          {/* Logo Identitaire - Rigoureux, Typographique */}
          <div className="flex items-center gap-3 font-semibold tracking-tight text-base">
            <div className="w-6 h-6 bg-primary flex items-center justify-center text-white font-mono font-bold text-sm">
              AO
            </div>
            <span>AgentOps</span>
          </div>
        </div>
        
        <div className="p-3 space-y-1 flex-1 overflow-y-auto">
          <div className="text-[10px] font-mono text-sidebar-foreground/40 uppercase tracking-widest mb-3 px-3 mt-4">Opérations</div>
          <NavItem icon={Inbox} label="Synthèse" id="dashboard" />
          <NavItem icon={Mail} label="À valider" id="validate" count={kpis.toValidate} />
          
          <div className="text-[10px] font-mono text-sidebar-foreground/40 uppercase tracking-widest mb-3 px-3 mt-8">Extraction</div>
          <NavItem icon={CheckCircle2} label="Engagements" id="engagements" />
          <NavItem icon={AlertTriangle} label="Alertes" id="alerts" count={kpis.atRisk} />
          
          <div className="text-[10px] font-mono text-sidebar-foreground/40 uppercase tracking-widest mb-3 px-3 mt-8">Système</div>
          <NavItem icon={ShieldCheck} label="Règles métier" id="agent" />
          <NavItem icon={Settings} label="Réglages" id="settings" />
        </div>

        <div className="p-4 border-t border-sidebar-border flex items-center justify-between">
          <div className="flex items-center gap-3 text-sm">
            <div className="w-8 h-8 bg-sidebar-border flex items-center justify-center">
              <User className="w-4 h-4 text-sidebar-foreground/60" />
            </div>
            <div className="flex flex-col">
              <span className="font-medium text-xs">J. Dupont</span>
              <span className="text-[10px] text-sidebar-foreground/40 font-mono">ID: 0x4B2</span>
            </div>
          </div>
          <button 
            onClick={() => setIsDarkMode(!isDarkMode)}
            className="text-sidebar-foreground/60 hover:text-sidebar-foreground p-2 hover:bg-sidebar-border/50 transition-colors"
          >
            {isDarkMode ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
          </button>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 flex flex-col h-screen overflow-hidden bg-background">
        {/* Topbar - Minimaliste */}
        <header className="h-14 border-b border-border flex items-center justify-between px-8 shrink-0 bg-card">
          <div className="flex items-center gap-3">
            <ScanText className="w-4 h-4 text-muted-foreground" />
            <h1 className="text-sm font-medium text-foreground">
              {currentView === 'dashboard' ? 'Synthèse' : currentView}
            </h1>
          </div>
          <div className="flex items-center gap-3">
            <div className="relative group w-72 hidden sm:block">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
              <input 
                type="text" 
                placeholder="Rechercher (Cmd + K)" 
                className="w-full h-8 pl-9 pr-10 text-xs border border-border bg-secondary/30 focus:bg-background focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary transition-all placeholder:text-muted-foreground"
              />
              <div className="absolute right-2 top-1/2 -translate-y-1/2 flex items-center gap-0.5">
                <Command className="w-3 h-3 text-muted-foreground" />
                <span className="text-[10px] text-muted-foreground font-mono">K</span>
              </div>
            </div>
          </div>
        </header>

        {/* Scrollable Area */}
        <div className="flex-1 overflow-auto p-8">
          <div className="max-w-5xl mx-auto space-y-10">
            
            {currentView === "dashboard" && (
              <>
                {/* L'identité Visuelle: Architecture de l'information stricte */}
                <div className="flex items-baseline justify-between mb-2">
                  <h2 className="text-2xl font-semibold tracking-tight">Vue d'ensemble</h2>
                  <span className="text-xs text-muted-foreground font-mono flex items-center gap-2">
                    <span className="w-1.5 h-1.5 bg-primary rounded-full animate-pulse"></span>
                    Agent actif
                  </span>
                </div>

                {/* KPIs - Blocs solides avec bordures fines, inspirés de tableaux de bord financiers */}
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                  {[
                    { label: "Actifs", value: kpis.active, color: "text-foreground", highlight: false },
                    { label: "En retard", value: kpis.delayed, color: "text-delay", highlight: false },
                    { label: "Critiques", value: kpis.atRisk, color: "text-risk", highlight: true },
                    { label: "À valider", value: kpis.toValidate, color: "text-primary", highlight: true },
                  ].map((stat, i) => (
                    <div key={i} className={cn(
                      "p-5 bg-card border flex flex-col gap-2 relative",
                      stat.highlight ? "border-border shadow-[4px_4px_0px_0px_rgba(0,0,0,0.05)] dark:shadow-[4px_4px_0px_0px_rgba(255,255,255,0.05)]" : "border-border"
                    )}>
                      {stat.highlight && <div className="absolute top-0 left-0 w-full h-0.5 bg-current" style={{ color: stat.color === 'text-primary' ? 'var(--primary)' : 'var(--risk)' }} />}
                      <span className="text-[10px] text-muted-foreground font-mono uppercase tracking-widest">{stat.label}</span>
                      <span className={cn("text-3xl font-light tracking-tighter font-mono", stat.color)}>
                        {stat.value}
                      </span>
                    </div>
                  ))}
                </div>

                {/* Triage Section - "Document style" */}
                <section>
                  <div className="flex items-center justify-between mb-4 mt-8">
                    <h2 className="text-sm font-semibold text-foreground uppercase tracking-wide">
                      Interventions Requises
                    </h2>
                    <Button variant="ghost" size="sm" className="h-7 text-xs font-mono">
                      [ TOUT_MARQUER_LU ]
                    </Button>
                  </div>
                  
                  <div className="border border-border bg-card">
                    <table className="w-full text-sm text-left">
                      <thead className="bg-secondary/50 text-muted-foreground text-[10px] font-mono uppercase tracking-widest border-b border-border">
                        <tr>
                          <th className="px-5 py-3 font-medium">Source / Tiers</th>
                          <th className="px-5 py-3 font-medium">Engagement Extrait</th>
                          <th className="px-5 py-3 font-medium w-1/4">Diagnostic</th>
                          <th className="px-5 py-3 font-medium text-right w-32">Action</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-border">
                        {mockEngagements.filter(e => e.status === 'risk' || e.status === 'delay').map(eng => (
                          <tr key={eng.id} className="hover:bg-secondary/20 transition-colors group relative">
                            {/* Visual indicator of "Extracted Data" */}
                            <td className="absolute left-0 top-0 bottom-0 w-1 bg-transparent group-hover:bg-primary transition-colors" />
                            
                            <td className="px-5 py-4 align-top">
                              <div className="font-semibold text-foreground mb-1">{eng.client}</div>
                              <div className="text-xs text-muted-foreground flex items-center gap-1.5 font-mono">
                                <FileText className="w-3 h-3" />
                                {eng.sourceEmail}
                              </div>
                            </td>
                            <td className="px-5 py-4 align-top">
                              <div className="flex items-center gap-2 mb-1">
                                <span className="text-xs font-medium text-foreground px-1.5 py-0.5 bg-secondary">{eng.type}</span>
                              </div>
                              <div className="text-muted-foreground">{eng.title}</div>
                            </td>
                            <td className="px-5 py-4 align-top">
                              <div className="flex flex-col gap-2 items-start">
                                <Badge status={eng.status}>{eng.status === 'risk' ? 'Risque' : 'Retard'}</Badge>
                                <span className="text-xs font-mono text-muted-foreground">{eng.blocker}</span>
                              </div>
                            </td>
                            <td className="px-5 py-4 align-top text-right">
                              <Button size="sm" variant="outline" onClick={() => openAction(eng)} className="w-full justify-between hover:border-primary hover:text-primary">
                                Agir
                                <ArrowRight className="w-3 h-3" />
                              </Button>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </section>
              </>
            )}

            {currentView === "engagements" && (
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <div className="relative w-72">
                    <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                    <input 
                      type="text" 
                      placeholder="Filtrer les engagements..." 
                      className="w-full h-9 pl-9 pr-3 text-xs border border-border bg-card focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                    />
                  </div>
                  <Button variant="outline" size="sm" className="gap-2 text-xs font-mono"><Filter className="w-3 h-3"/> Vues</Button>
                </div>
                <div className="border border-border bg-card">
                  <table className="w-full text-sm text-left">
                    <thead className="bg-secondary/50 text-muted-foreground text-[10px] font-mono uppercase tracking-widest border-b border-border">
                      <tr>
                        <th className="px-5 py-3 font-medium w-8">État</th>
                        <th className="px-5 py-3 font-medium">Tiers</th>
                        <th className="px-5 py-3 font-medium">Engagement Extrait</th>
                        <th className="px-5 py-3 font-medium">Échéance</th>
                        <th className="px-5 py-3 font-medium text-right">Actions</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-border">
                      {mockEngagements.map(eng => (
                        <tr key={eng.id} className="hover:bg-secondary/20 transition-colors">
                          <td className="px-5 py-3">
                            <div className={cn("w-2 h-2 rounded-none", eng.status === 'risk' ? 'bg-risk' : eng.status === 'delay' ? 'bg-delay' : 'bg-healthy')} />
                          </td>
                          <td className="px-5 py-3 font-semibold text-foreground">{eng.client}</td>
                          <td className="px-5 py-3">
                            <div className="text-foreground mb-0.5">{eng.title}</div>
                            <div className="text-xs text-muted-foreground font-mono">{eng.sourceEmail}</div>
                          </td>
                          <td className="px-5 py-3 text-xs font-mono text-muted-foreground">
                            {eng.deadline}
                          </td>
                          <td className="px-5 py-3 text-right">
                            <Button variant="ghost" size="sm" onClick={() => openAction(eng)}>
                              Ouvrir
                            </Button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}
          </div>
        </div>
      </main>

      {/* Action Validation Drawer - Sharp and authoritative */}
      <Dialog.Root open={drawerOpen} onOpenChange={setDrawerOpen}>
        <Dialog.Portal>
          <Dialog.Overlay className="fixed inset-0 bg-background/80 backdrop-blur-sm z-40 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0" />
          <Dialog.Content className="fixed right-0 top-0 bottom-0 w-full max-w-[480px] bg-card border-l border-border shadow-2xl z-50 flex flex-col data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:slide-out-to-right data-[state=open]:slide-in-from-right duration-200">
            
            <div className="h-14 border-b border-border flex items-center justify-between px-6 shrink-0 bg-secondary/30">
              <Dialog.Title className="text-sm font-semibold flex items-center gap-3">
                <div className="w-2 h-2 bg-primary"></div>
                Préparation du suivi
              </Dialog.Title>
              <Dialog.Close asChild>
                <button className="p-1 hover:bg-secondary text-muted-foreground hover:text-foreground">
                  <X className="w-4 h-4" />
                </button>
              </Dialog.Close>
            </div>

            {selectedEngagement && (
              <div className="flex-1 overflow-auto p-6 space-y-8">
                
                {/* Context Block - Looks like extracted metadata */}
                <div className="space-y-2">
                  <div className="text-[10px] font-mono text-muted-foreground uppercase tracking-widest">Métadonnées extraites</div>
                  <div className="border border-border p-4 bg-background">
                    <div className="grid grid-cols-2 gap-y-3 text-sm">
                      <div className="text-muted-foreground font-mono text-xs">Tiers</div>
                      <div className="font-semibold">{selectedEngagement.client}</div>
                      
                      <div className="text-muted-foreground font-mono text-xs">Sujet</div>
                      <div>{selectedEngagement.title}</div>
                      
                      <div className="text-muted-foreground font-mono text-xs">Échéance</div>
                      <div className="font-mono">{selectedEngagement.deadline}</div>
                      
                      <div className="text-muted-foreground font-mono text-xs">Alerte</div>
                      <div className="text-risk font-medium">{selectedEngagement.blocker}</div>
                    </div>
                  </div>
                </div>

                {/* Email Draft Form - Like a rigid form */}
                <div className="space-y-4">
                  <div className="text-[10px] font-mono text-muted-foreground uppercase tracking-widest">Brouillon proposé</div>
                  
                  <div className="border border-border bg-background">
                    <div className="flex items-center border-b border-border p-3">
                      <label className="text-xs font-mono text-muted-foreground w-16">À</label>
                      <input 
                        type="text" 
                        defaultValue={`contact@${selectedEngagement.client.toLowerCase().replace(' ', '')}.com`}
                        className="flex-1 text-sm bg-transparent border-none focus:outline-none focus:ring-0 text-foreground font-medium"
                      />
                    </div>
                    <div className="flex items-center border-b border-border p-3">
                      <label className="text-xs font-mono text-muted-foreground w-16">Objet</label>
                      <input 
                        type="text" 
                        defaultValue={`Suivi : ${selectedEngagement.title}`}
                        className="flex-1 text-sm bg-transparent border-none focus:outline-none focus:ring-0 text-foreground"
                      />
                    </div>
                    <div>
                      <textarea 
                        className="w-full min-h-[220px] p-4 text-sm bg-transparent border-none focus:outline-none focus:ring-0 resize-y leading-relaxed text-foreground"
                        defaultValue={`Bonjour,\n\nJe me permets de revenir vers vous concernant ${selectedEngagement.title}.\nSauf erreur de ma part, nous avions convenu d'une échéance au ${selectedEngagement.deadline}.\n\nPourriez-vous me faire un retour rapide pour que nous puissions avancer sur ce dossier ?\n\nBien à vous,\nJ. Dupont`}
                      />
                    </div>
                  </div>
                </div>
              </div>
            )}

            <div className="p-5 border-t border-border bg-card shrink-0 flex items-center justify-between">
              <span className="text-xs text-muted-foreground flex items-center gap-2 font-mono">
                Action manuelle requise
              </span>
              <div className="flex gap-2">
                <Button variant="ghost" onClick={() => setDrawerOpen(false)}>Annuler</Button>
                <Button className="gap-2 px-6 bg-primary text-primary-foreground hover:bg-primary/90">
                  Valider
                  <Send className="w-3.5 h-3.5" />
                </Button>
              </div>
            </div>
            
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>

    </div>
  );
}