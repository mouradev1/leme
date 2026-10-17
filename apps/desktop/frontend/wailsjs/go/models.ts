export namespace api {
	
	export class MetricsQuery {
	    scope: string;
	    namespace: string;
	    intervalSeconds: number;
	    promNamespace: string;
	    promService: string;
	    promPort: string;
	
	    static createFrom(source: any = {}) {
	        return new MetricsQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scope = source["scope"];
	        this.namespace = source["namespace"];
	        this.intervalSeconds = source["intervalSeconds"];
	        this.promNamespace = source["promNamespace"];
	        this.promService = source["promService"];
	        this.promPort = source["promPort"];
	    }
	}
	export class PodContainers {
	    containers: string[];
	    initContainers: string[];
	
	    static createFrom(source: any = {}) {
	        return new PodContainers(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.containers = source["containers"];
	        this.initContainers = source["initContainers"];
	    }
	}
	export class Prefs {
	    updateCheck: boolean;
	    skippedVersion: string;
	
	    static createFrom(source: any = {}) {
	        return new Prefs(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.updateCheck = source["updateCheck"];
	        this.skippedVersion = source["skippedVersion"];
	    }
	}
	export class ServerInfo {
	    context: string;
	    version: string;
	    server: string;
	    defaultNamespace: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.context = source["context"];
	        this.version = source["version"];
	        this.server = source["server"];
	        this.defaultNamespace = source["defaultNamespace"];
	    }
	}

}

export namespace cluster {
	
	export class ContextInfo {
	    name: string;
	    cluster: string;
	    user: string;
	    namespace: string;
	    server: string;
	    auth: string;
	    source: string;
	    current: boolean;
	    connected: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ContextInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.cluster = source["cluster"];
	        this.user = source["user"];
	        this.namespace = source["namespace"];
	        this.server = source["server"];
	        this.auth = source["auth"];
	        this.source = source["source"];
	        this.current = source["current"];
	        this.connected = source["connected"];
	    }
	}
	export class KubeconfigFile {
	    path: string;
	    contexts: number;
	    exists: boolean;
	    removable: boolean;
	    managed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new KubeconfigFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.contexts = source["contexts"];
	        this.exists = source["exists"];
	        this.removable = source["removable"];
	        this.managed = source["managed"];
	    }
	}
	export class SavedKubeconfig {
	    path: string;
	    contexts: string[];
	    shadowed: string[];
	
	    static createFrom(source: any = {}) {
	        return new SavedKubeconfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.contexts = source["contexts"];
	        this.shadowed = source["shadowed"];
	    }
	}

}

export namespace helm {
	
	export class InstallRequest {
	    releaseName: string;
	    namespace: string;
	    chart: string;
	    repoURL: string;
	    version: string;
	    valuesYAML: string;
	    createNamespace: boolean;
	    wait: boolean;
	
	    static createFrom(source: any = {}) {
	        return new InstallRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.releaseName = source["releaseName"];
	        this.namespace = source["namespace"];
	        this.chart = source["chart"];
	        this.repoURL = source["repoURL"];
	        this.version = source["version"];
	        this.valuesYAML = source["valuesYAML"];
	        this.createNamespace = source["createNamespace"];
	        this.wait = source["wait"];
	    }
	}
	export class Release {
	    name: string;
	    namespace: string;
	    revision: number;
	    status: string;
	    chart: string;
	    chartVersion: string;
	    appVersion: string;
	    description: string;
	    updated: string;
	
	    static createFrom(source: any = {}) {
	        return new Release(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.revision = source["revision"];
	        this.status = source["status"];
	        this.chart = source["chart"];
	        this.chartVersion = source["chartVersion"];
	        this.appVersion = source["appVersion"];
	        this.description = source["description"];
	        this.updated = source["updated"];
	    }
	}

}

export namespace ops {
	
	export class DrainOptions {
	    force: boolean;
	    deleteEmptyDirData: boolean;
	    gracePeriodSeconds: number;
	    timeoutSeconds: number;
	    disableEviction: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DrainOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.force = source["force"];
	        this.deleteEmptyDirData = source["deleteEmptyDirData"];
	        this.gracePeriodSeconds = source["gracePeriodSeconds"];
	        this.timeoutSeconds = source["timeoutSeconds"];
	        this.disableEviction = source["disableEviction"];
	    }
	}

}

export namespace pods {
	
	export class Condition {
	    type: string;
	    status: string;
	    reason: string;
	    message: string;
	    lastTransition: string;
	
	    static createFrom(source: any = {}) {
	        return new Condition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.status = source["status"];
	        this.reason = source["reason"];
	        this.message = source["message"];
	        this.lastTransition = source["lastTransition"];
	    }
	}
	export class Resources {
	    cpu: number;
	    memory: number;
	
	    static createFrom(source: any = {}) {
	        return new Resources(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cpu = source["cpu"];
	        this.memory = source["memory"];
	    }
	}
	export class Probe {
	    handler: string;
	    target: string;
	    delay: number;
	    timeout: number;
	    period: number;
	    success: number;
	    failure: number;
	
	    static createFrom(source: any = {}) {
	        return new Probe(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.handler = source["handler"];
	        this.target = source["target"];
	        this.delay = source["delay"];
	        this.timeout = source["timeout"];
	        this.period = source["period"];
	        this.success = source["success"];
	        this.failure = source["failure"];
	    }
	}
	export class Mount {
	    path: string;
	    volume: string;
	    readOnly: boolean;
	    subPath: string;
	
	    static createFrom(source: any = {}) {
	        return new Mount(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.volume = source["volume"];
	        this.readOnly = source["readOnly"];
	        this.subPath = source["subPath"];
	    }
	}
	export class EnvVar {
	    name: string;
	    value: string;
	    from: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvVar(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.value = source["value"];
	        this.from = source["from"];
	    }
	}
	export class Port {
	    name: string;
	    containerPort: number;
	    protocol: string;
	
	    static createFrom(source: any = {}) {
	        return new Port(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.containerPort = source["containerPort"];
	        this.protocol = source["protocol"];
	    }
	}
	export class Termination {
	    reason: string;
	    exitCode: number;
	    signal: number;
	    message: string;
	    startedAt: string;
	    finishedAt: string;
	    meaning: string;
	
	    static createFrom(source: any = {}) {
	        return new Termination(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.reason = source["reason"];
	        this.exitCode = source["exitCode"];
	        this.signal = source["signal"];
	        this.message = source["message"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.meaning = source["meaning"];
	    }
	}
	export class Container {
	    name: string;
	    init: boolean;
	    image: string;
	    imagePullPolicy: string;
	    state: string;
	    stateReason: string;
	    stateMessage: string;
	    startedAt: string;
	    ready: boolean;
	    restartCount: number;
	    lastTermination?: Termination;
	    ports: Port[];
	    env: EnvVar[];
	    envFrom: string[];
	    mounts: Mount[];
	    liveness?: Probe;
	    readiness?: Probe;
	    startup?: Probe;
	    requests: Resources;
	    limits: Resources;
	
	    static createFrom(source: any = {}) {
	        return new Container(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.init = source["init"];
	        this.image = source["image"];
	        this.imagePullPolicy = source["imagePullPolicy"];
	        this.state = source["state"];
	        this.stateReason = source["stateReason"];
	        this.stateMessage = source["stateMessage"];
	        this.startedAt = source["startedAt"];
	        this.ready = source["ready"];
	        this.restartCount = source["restartCount"];
	        this.lastTermination = this.convertValues(source["lastTermination"], Termination);
	        this.ports = this.convertValues(source["ports"], Port);
	        this.env = this.convertValues(source["env"], EnvVar);
	        this.envFrom = source["envFrom"];
	        this.mounts = this.convertValues(source["mounts"], Mount);
	        this.liveness = this.convertValues(source["liveness"], Probe);
	        this.readiness = this.convertValues(source["readiness"], Probe);
	        this.startup = this.convertValues(source["startup"], Probe);
	        this.requests = this.convertValues(source["requests"], Resources);
	        this.limits = this.convertValues(source["limits"], Resources);
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
	export class Diagnosis {
	    severity: string;
	    container: string;
	    title: string;
	    detail: string;
	    hint: string;
	
	    static createFrom(source: any = {}) {
	        return new Diagnosis(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.severity = source["severity"];
	        this.container = source["container"];
	        this.title = source["title"];
	        this.detail = source["detail"];
	        this.hint = source["hint"];
	    }
	}
	export class Volume {
	    name: string;
	    kind: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new Volume(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.source = source["source"];
	    }
	}
	export class Details {
	    phase: string;
	    reason: string;
	    message: string;
	    node: string;
	    podIP: string;
	    podIPs: string[];
	    hostIP: string;
	    serviceAccount: string;
	    qosClass: string;
	    priorityClass: string;
	    restartPolicy: string;
	    startTime: string;
	    conditions: Condition[];
	    tolerations: string[];
	    nodeSelector: Record<string, string>;
	    volumes: Volume[];
	    initContainers: Container[];
	    containers: Container[];
	    diagnoses: Diagnosis[];
	
	    static createFrom(source: any = {}) {
	        return new Details(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.phase = source["phase"];
	        this.reason = source["reason"];
	        this.message = source["message"];
	        this.node = source["node"];
	        this.podIP = source["podIP"];
	        this.podIPs = source["podIPs"];
	        this.hostIP = source["hostIP"];
	        this.serviceAccount = source["serviceAccount"];
	        this.qosClass = source["qosClass"];
	        this.priorityClass = source["priorityClass"];
	        this.restartPolicy = source["restartPolicy"];
	        this.startTime = source["startTime"];
	        this.conditions = this.convertValues(source["conditions"], Condition);
	        this.tolerations = source["tolerations"];
	        this.nodeSelector = source["nodeSelector"];
	        this.volumes = this.convertValues(source["volumes"], Volume);
	        this.initContainers = this.convertValues(source["initContainers"], Container);
	        this.containers = this.convertValues(source["containers"], Container);
	        this.diagnoses = this.convertValues(source["diagnoses"], Diagnosis);
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

export namespace resources {
	
	export class ApplyOptions {
	    dryRun: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ApplyOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dryRun = source["dryRun"];
	    }
	}
	export class ApplyResult {
	    yaml: string;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new ApplyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.yaml = source["yaml"];
	        this.warnings = source["warnings"];
	    }
	}
	export class Kind {
	    group: string;
	    version: string;
	    resource: string;
	    namespaced: boolean;
	    kind: string;
	    category: string;
	    verbs: string[];
	    shortNames: string[];
	
	    static createFrom(source: any = {}) {
	        return new Kind(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.group = source["group"];
	        this.version = source["version"];
	        this.resource = source["resource"];
	        this.namespaced = source["namespaced"];
	        this.kind = source["kind"];
	        this.category = source["category"];
	        this.verbs = source["verbs"];
	        this.shortNames = source["shortNames"];
	    }
	}
	export class ObjectData {
	    data: Record<string, string>;
	    binary: string[];
	    resourceVersion: string;
	
	    static createFrom(source: any = {}) {
	        return new ObjectData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.data = source["data"];
	        this.binary = source["binary"];
	        this.resourceVersion = source["resourceVersion"];
	    }
	}
	export class Query {
	    namespace: string;
	    labelSelector: string;
	    fieldSelector: string;
	
	    static createFrom(source: any = {}) {
	        return new Query(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.namespace = source["namespace"];
	        this.labelSelector = source["labelSelector"];
	        this.fieldSelector = source["fieldSelector"];
	    }
	}
	export class Ref {
	    group: string;
	    version: string;
	    resource: string;
	    namespaced: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Ref(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.group = source["group"];
	        this.version = source["version"];
	        this.resource = source["resource"];
	        this.namespaced = source["namespaced"];
	    }
	}

}

export namespace stream {
	
	export class LogTarget {
	    pod: string;
	    container: string;
	
	    static createFrom(source: any = {}) {
	        return new LogTarget(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pod = source["pod"];
	        this.container = source["container"];
	    }
	}
	export class LogOptions {
	    namespace: string;
	    targets: LogTarget[];
	    tailLines: number;
	    sinceSeconds: number;
	    timestamps: boolean;
	    previous: boolean;
	    follow: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LogOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.namespace = source["namespace"];
	        this.targets = this.convertValues(source["targets"], LogTarget);
	        this.tailLines = source["tailLines"];
	        this.sinceSeconds = source["sinceSeconds"];
	        this.timestamps = source["timestamps"];
	        this.previous = source["previous"];
	        this.follow = source["follow"];
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

export namespace terminal {
	
	export class ExecTarget {
	    namespace: string;
	    pod: string;
	    container: string;
	    command: string[];
	
	    static createFrom(source: any = {}) {
	        return new ExecTarget(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.namespace = source["namespace"];
	        this.pod = source["pod"];
	        this.container = source["container"];
	        this.command = source["command"];
	    }
	}

}

export namespace update {
	
	export class Info {
	    current: string;
	    latest: string;
	    available: boolean;
	    url: string;
	    notes: string;
	    published: string;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = source["current"];
	        this.latest = source["latest"];
	        this.available = source["available"];
	        this.url = source["url"];
	        this.notes = source["notes"];
	        this.published = source["published"];
	    }
	}

}

