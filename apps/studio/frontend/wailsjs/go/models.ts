export namespace environment {
	
	export class Snapshot {
	    configPath: string;
	    configPresent: boolean;
	    defaultAppId?: string;
	    keychainAvailable: boolean;
	    keychainBypassed: boolean;
	    keychainWarning?: string;
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
	        this.keychainWarning = source["keychainWarning"];
	        this.workflowPath = source["workflowPath"];
	    }
	}

}

export namespace main {
	
	export class ASCCommandResponse {
	    data: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ASCCommandResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.data = source["data"];
	        this.error = source["error"];
	    }
	}
	export class AppVersion {
	    id: string;
	    platform: string;
	    version: string;
	    state: string;
	
	    static createFrom(source: any = {}) {
	        return new AppVersion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.platform = source["platform"];
	        this.version = source["version"];
	        this.state = source["state"];
	    }
	}
	export class AppDetail {
	    id: string;
	    name: string;
	    subtitle: string;
	    bundleId: string;
	    sku: string;
	    primaryLocale: string;
	    versions: AppVersion[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new AppDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.subtitle = source["subtitle"];
	        this.bundleId = source["bundleId"];
	        this.sku = source["sku"];
	        this.primaryLocale = source["primaryLocale"];
	        this.versions = this.convertValues(source["versions"], AppVersion);
	        this.error = source["error"];
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
	export class AppInfo {
	    id: string;
	    name: string;
	    subtitle: string;
	    bundleId: string;
	    platform: string;
	    sku: string;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.subtitle = source["subtitle"];
	        this.bundleId = source["bundleId"];
	        this.platform = source["platform"];
	        this.sku = source["sku"];
	    }
	}
	export class AppLocalization {
	    localizationId: string;
	    locale: string;
	    description: string;
	    keywords: string;
	    whatsNew: string;
	    promotionalText: string;
	    supportUrl: string;
	    marketingUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new AppLocalization(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.localizationId = source["localizationId"];
	        this.locale = source["locale"];
	        this.description = source["description"];
	        this.keywords = source["keywords"];
	        this.whatsNew = source["whatsNew"];
	        this.promotionalText = source["promotionalText"];
	        this.supportUrl = source["supportUrl"];
	        this.marketingUrl = source["marketingUrl"];
	    }
	}
	export class AppScreenshot {
	    thumbnailUrl: string;
	    width: number;
	    height: number;
	
	    static createFrom(source: any = {}) {
	        return new AppScreenshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.thumbnailUrl = source["thumbnailUrl"];
	        this.width = source["width"];
	        this.height = source["height"];
	    }
	}
	
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
	export class BetaGroup {
	    id: string;
	    name: string;
	    isInternal: boolean;
	    publicLink: string;
	    feedbackEnabled: boolean;
	    createdDate: string;
	    testerCount: number;
	
	    static createFrom(source: any = {}) {
	        return new BetaGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.isInternal = source["isInternal"];
	        this.publicLink = source["publicLink"];
	        this.feedbackEnabled = source["feedbackEnabled"];
	        this.createdDate = source["createdDate"];
	        this.testerCount = source["testerCount"];
	    }
	}
	export class BetaTester {
	    email: string;
	    firstName: string;
	    lastName: string;
	    inviteType: string;
	    state: string;
	
	    static createFrom(source: any = {}) {
	        return new BetaTester(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.email = source["email"];
	        this.firstName = source["firstName"];
	        this.lastName = source["lastName"];
	        this.inviteType = source["inviteType"];
	        this.state = source["state"];
	    }
	}
	export class StudioApproval {
	    id: string;
	    threadId: string;
	    title: string;
	    summary: string;
	    commandPreview: string[];
	    mutationSurface: string;
	    status: string;
	    createdAt: string;
	    resolvedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new StudioApproval(source);
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
	        this.createdAt = source["createdAt"];
	        this.resolvedAt = source["resolvedAt"];
	    }
	}
	export class StudioMessage {
	    id: string;
	    role: string;
	    kind: string;
	    content: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new StudioMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.role = source["role"];
	        this.kind = source["kind"];
	        this.content = source["content"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class StudioThread {
	    id: string;
	    title: string;
	    sessionId?: string;
	    createdAt: string;
	    updatedAt: string;
	    messages: StudioMessage[];
	
	    static createFrom(source: any = {}) {
	        return new StudioThread(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.sessionId = source["sessionId"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.messages = this.convertValues(source["messages"], StudioMessage);
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
	    generatedAt: string;
	    sections: WorkspaceSection[];
	    settings: settings.StudioSettings;
	    presets: settings.ProviderPreset[];
	    environment: environment.Snapshot;
	    threads: StudioThread[];
	    approvals: StudioApproval[];
	    windowFlavor: string;
	
	    static createFrom(source: any = {}) {
	        return new BootstrapData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appName = source["appName"];
	        this.tagline = source["tagline"];
	        this.generatedAt = source["generatedAt"];
	        this.sections = this.convertValues(source["sections"], WorkspaceSection);
	        this.settings = this.convertValues(source["settings"], settings.StudioSettings);
	        this.presets = this.convertValues(source["presets"], settings.ProviderPreset);
	        this.environment = this.convertValues(source["environment"], environment.Snapshot);
	        this.threads = this.convertValues(source["threads"], StudioThread);
	        this.approvals = this.convertValues(source["approvals"], StudioApproval);
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
	export class FeedbackScreenshot {
	    url: string;
	    width: number;
	    height: number;
	
	    static createFrom(source: any = {}) {
	        return new FeedbackScreenshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.width = source["width"];
	        this.height = source["height"];
	    }
	}
	export class FeedbackItem {
	    id: string;
	    comment: string;
	    email: string;
	    deviceModel: string;
	    deviceFamily: string;
	    osVersion: string;
	    appPlatform: string;
	    createdDate: string;
	    locale: string;
	    timeZone: string;
	    connectionType: string;
	    batteryPercentage: number;
	    screenshots: FeedbackScreenshot[];
	
	    static createFrom(source: any = {}) {
	        return new FeedbackItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.comment = source["comment"];
	        this.email = source["email"];
	        this.deviceModel = source["deviceModel"];
	        this.deviceFamily = source["deviceFamily"];
	        this.osVersion = source["osVersion"];
	        this.appPlatform = source["appPlatform"];
	        this.createdDate = source["createdDate"];
	        this.locale = source["locale"];
	        this.timeZone = source["timeZone"];
	        this.connectionType = source["connectionType"];
	        this.batteryPercentage = source["batteryPercentage"];
	        this.screenshots = this.convertValues(source["screenshots"], FeedbackScreenshot);
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
	export class FeedbackResponse {
	    feedback: FeedbackItem[];
	    total: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new FeedbackResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.feedback = this.convertValues(source["feedback"], FeedbackItem);
	        this.total = source["total"];
	        this.error = source["error"];
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
	
	export class FinanceRegion {
	    reportRegion: string;
	    reportCurrency: string;
	    regionCode: string;
	    countriesOrRegions: string;
	
	    static createFrom(source: any = {}) {
	        return new FinanceRegion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.reportRegion = source["reportRegion"];
	        this.reportCurrency = source["reportCurrency"];
	        this.regionCode = source["regionCode"];
	        this.countriesOrRegions = source["countriesOrRegions"];
	    }
	}
	export class FinanceResponse {
	    regions: FinanceRegion[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new FinanceResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.regions = this.convertValues(source["regions"], FinanceRegion);
	        this.error = source["error"];
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
	export class ListAppsResponse {
	    apps: AppInfo[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ListAppsResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.apps = this.convertValues(source["apps"], AppInfo);
	        this.error = source["error"];
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
	export class OfferCode {
	    subscriptionName: string;
	    subscriptionId: string;
	    name: string;
	    offerEligibility: string;
	    customerEligibilities: string[];
	    duration: string;
	    offerMode: string;
	    numberOfPeriods: number;
	    totalNumberOfCodes: number;
	    productionCodeCount: number;
	
	    static createFrom(source: any = {}) {
	        return new OfferCode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.subscriptionName = source["subscriptionName"];
	        this.subscriptionId = source["subscriptionId"];
	        this.name = source["name"];
	        this.offerEligibility = source["offerEligibility"];
	        this.customerEligibilities = source["customerEligibilities"];
	        this.duration = source["duration"];
	        this.offerMode = source["offerMode"];
	        this.numberOfPeriods = source["numberOfPeriods"];
	        this.totalNumberOfCodes = source["totalNumberOfCodes"];
	        this.productionCodeCount = source["productionCodeCount"];
	    }
	}
	export class OfferCodesResponse {
	    offerCodes: OfferCode[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new OfferCodesResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.offerCodes = this.convertValues(source["offerCodes"], OfferCode);
	        this.error = source["error"];
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
	export class SubPricingItem {
	    name: string;
	    productId: string;
	    subscriptionPeriod: string;
	    state: string;
	    groupName: string;
	    price: string;
	    currency: string;
	    proceeds: string;
	
	    static createFrom(source: any = {}) {
	        return new SubPricingItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.productId = source["productId"];
	        this.subscriptionPeriod = source["subscriptionPeriod"];
	        this.state = source["state"];
	        this.groupName = source["groupName"];
	        this.price = source["price"];
	        this.currency = source["currency"];
	        this.proceeds = source["proceeds"];
	    }
	}
	export class TerritoryAvailability {
	    territory: string;
	    available: boolean;
	    releaseDate: string;
	
	    static createFrom(source: any = {}) {
	        return new TerritoryAvailability(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.territory = source["territory"];
	        this.available = source["available"];
	        this.releaseDate = source["releaseDate"];
	    }
	}
	export class PricingOverview {
	    availableInNewTerritories: boolean;
	    currentPrice: string;
	    currentProceeds: string;
	    baseCurrency: string;
	    territories: TerritoryAvailability[];
	    subscriptionPricing: SubPricingItem[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new PricingOverview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.availableInNewTerritories = source["availableInNewTerritories"];
	        this.currentPrice = source["currentPrice"];
	        this.currentProceeds = source["currentProceeds"];
	        this.baseCurrency = source["baseCurrency"];
	        this.territories = this.convertValues(source["territories"], TerritoryAvailability);
	        this.subscriptionPricing = this.convertValues(source["subscriptionPricing"], SubPricingItem);
	        this.error = source["error"];
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
	    thread: StudioThread;
	
	    static createFrom(source: any = {}) {
	        return new PromptResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.thread = this.convertValues(source["thread"], StudioThread);
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
	export class ScreenshotSet {
	    displayType: string;
	    screenshots: AppScreenshot[];
	
	    static createFrom(source: any = {}) {
	        return new ScreenshotSet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.displayType = source["displayType"];
	        this.screenshots = this.convertValues(source["screenshots"], AppScreenshot);
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
	export class ScreenshotsResponse {
	    sets: ScreenshotSet[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ScreenshotsResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sets = this.convertValues(source["sets"], ScreenshotSet);
	        this.error = source["error"];
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
	
	
	
	
	export class SubscriptionItem {
	    id: string;
	    groupName: string;
	    name: string;
	    productId: string;
	    state: string;
	    subscriptionPeriod: string;
	    reviewNote: string;
	    groupLevel: number;
	
	    static createFrom(source: any = {}) {
	        return new SubscriptionItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.groupName = source["groupName"];
	        this.name = source["name"];
	        this.productId = source["productId"];
	        this.state = source["state"];
	        this.subscriptionPeriod = source["subscriptionPeriod"];
	        this.reviewNote = source["reviewNote"];
	        this.groupLevel = source["groupLevel"];
	    }
	}
	export class SubscriptionsResponse {
	    subscriptions: SubscriptionItem[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new SubscriptionsResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.subscriptions = this.convertValues(source["subscriptions"], SubscriptionItem);
	        this.error = source["error"];
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
	
	export class TestFlightResponse {
	    groups: BetaGroup[];
	    testers: BetaTester[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new TestFlightResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.groups = this.convertValues(source["groups"], BetaGroup);
	        this.testers = this.convertValues(source["testers"], BetaTester);
	        this.error = source["error"];
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
	export class VersionMetadataResponse {
	    localizations: AppLocalization[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new VersionMetadataResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.localizations = this.convertValues(source["localizations"], AppLocalization);
	        this.error = source["error"];
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
