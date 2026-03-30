export namespace approvals {
	
	export class Action {
	    id: string;
	    threadId: string;
	    title: string;
	    summary: string;
	    commandPreview: string[];
	    mutationSurface: string;
	    status: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    resolvedAt?: any;
	
	    static createFrom(source: any = {}) {
	        return new Action(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.threadId = source["threadId"];
	        this.title = source["title"];
	        this.summary = source["summary"];
	        this.commandPreview = source["commandPreview"];
	        this.mutationSurface = source["mutationSurface"];
	        this.status = source["status"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.resolvedAt = this.convertValues(source["resolvedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace environment {
	
	export class Snapshot {
	    configPath: string;
	    configPresent: boolean;
	    defaultAppId?: string;
	    keychainAvailable: boolean;
	    keychainBypassed: boolean;
	    workflowPath: string;
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configPath = source["configPath"];
	        this.configPresent = source["configPresent"];
	        this.defaultAppId = source["defaultAppId"];
	        this.keychainAvailable = source["keychainAvailable"];
	        this.keychainBypassed = source["keychainBypassed"];
	        this.workflowPath = source["workflowPath"];
	    }
	}

}

export namespace main {
	
	export class ApprovalRequest {
	    threadId: string;
	    title: string;
	    summary: string;
	    commandPreview: string[];
	    mutationSurface: string;
	
	    static createFrom(source: any = {}) {
	        return new ApprovalRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.threadId = source["threadId"];
	        this.title = source["title"];
	        this.summary = source["summary"];
	        this.commandPreview = source["commandPreview"];
	        this.mutationSurface = source["mutationSurface"];
	    }
	}
	export class AuthStatus {
	    authenticated: boolean;
	    storage: string;
	    profile: string;
	    rawOutput: string;
	
	    static createFrom(source: any = {}) {
	        return new AuthStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.authenticated = source["authenticated"];
	        this.storage = source["storage"];
	        this.profile = source["profile"];
	        this.rawOutput = source["rawOutput"];
	    }
	}
	export class WorkspaceSection {
	    id: string;
	    label: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceSection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.description = source["description"];
	    }
	}
	export class BootstrapData {
	    appName: string;
	    tagline: string;
	    // Go type: time
	    generatedAt: any;
	    sections: WorkspaceSection[];
	    settings: settings.StudioSettings;
	    presets: settings.ProviderPreset[];
	    environment: environment.Snapshot;
	    threads: threads.Thread[];
	    approvals: approvals.Action[];
	    windowFlavor: string;
	
	    static createFrom(source: any = {}) {
	        return new BootstrapData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appName = source["appName"];
	        this.tagline = source["tagline"];
	        this.generatedAt = this.convertValues(source["generatedAt"], null);
	        this.sections = this.convertValues(source["sections"], WorkspaceSection);
	        this.settings = this.convertValues(source["settings"], settings.StudioSettings);
	        this.presets = this.convertValues(source["presets"], settings.ProviderPreset);
	        this.environment = this.convertValues(source["environment"], environment.Snapshot);
	        this.threads = this.convertValues(source["threads"], threads.Thread);
	        this.approvals = this.convertValues(source["approvals"], approvals.Action);
	        this.windowFlavor = source["windowFlavor"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PromptRequest {
	    threadId: string;
	    prompt: string;
	
	    static createFrom(source: any = {}) {
	        return new PromptRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.threadId = source["threadId"];
	        this.prompt = source["prompt"];
	    }
	}
	export class PromptResponse {
	    thread: threads.Thread;
	
	    static createFrom(source: any = {}) {
	        return new PromptResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.thread = this.convertValues(source["thread"], threads.Thread);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ResolutionResponse {
	    path: string;
	    source: string;
	    checked: string[];
	    bundledEligible: boolean;
	    availablePresets: settings.ProviderPreset[];
	
	    static createFrom(source: any = {}) {
	        return new ResolutionResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.source = source["source"];
	        this.checked = source["checked"];
	        this.bundledEligible = source["bundledEligible"];
	        this.availablePresets = this.convertValues(source["availablePresets"], settings.ProviderPreset);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace settings {
	
	export class ProviderPreset {
	    id: string;
	    name: string;
	    description: string;
	    suggestedCommand: string;
	    suggestedArgs: string[];
	
	    static createFrom(source: any = {}) {
	        return new ProviderPreset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.suggestedCommand = source["suggestedCommand"];
	        this.suggestedArgs = source["suggestedArgs"];
	    }
	}
	export class StudioSettings {
	    preferredPreset: string;
	    agentCommand: string;
	    agentArgs: string[];
	    agentEnv: Record<string, string>;
	    preferBundledASC: boolean;
	    systemASCPath: string;
	    workspaceRoot: string;
	    theme: string;
	    windowMaterial: string;
	    showCommandPreviews: boolean;
	
	    static createFrom(source: any = {}) {
	        return new StudioSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.preferredPreset = source["preferredPreset"];
	        this.agentCommand = source["agentCommand"];
	        this.agentArgs = source["agentArgs"];
	        this.agentEnv = source["agentEnv"];
	        this.preferBundledASC = source["preferBundledASC"];
	        this.systemASCPath = source["systemASCPath"];
	        this.workspaceRoot = source["workspaceRoot"];
	        this.theme = source["theme"];
	        this.windowMaterial = source["windowMaterial"];
	        this.showCommandPreviews = source["showCommandPreviews"];
	    }
	}

}

export namespace threads {
	
	export class Message {
	    id: string;
	    role: string;
	    kind: string;
	    content: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.role = source["role"];
	        this.kind = source["kind"];
	        this.content = source["content"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Thread {
	    id: string;
	    title: string;
	    sessionId?: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    messages: Message[];
	
	    static createFrom(source: any = {}) {
	        return new Thread(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.sessionId = source["sessionId"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.messages = this.convertValues(source["messages"], Message);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

