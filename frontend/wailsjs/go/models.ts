export namespace main {
	
	export class ConnectRequest {
	    sessionId: number;
	    password: string;
	    keyPEM: string;
	    keyPath: string;
	    proxyPassword: string;
	    cols: number;
	    rows: number;
	
	    static createFrom(source: any = {}) {
	        return new ConnectRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.password = source["password"];
	        this.keyPEM = source["keyPEM"];
	        this.keyPath = source["keyPath"];
	        this.proxyPassword = source["proxyPassword"];
	        this.cols = source["cols"];
	        this.rows = source["rows"];
	    }
	}
	export class HostKeyPrompt {
	    host: string;
	    keyType: string;
	    fingerprint: string;
	    blob: string;
	    changed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HostKeyPrompt(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.keyType = source["keyType"];
	        this.fingerprint = source["fingerprint"];
	        this.blob = source["blob"];
	        this.changed = source["changed"];
	    }
	}
	export class ConnectResult {
	    id: string;
	    host: string;
	    name: string;
	    sessionId: number;
	    hostKeyPrompt?: HostKeyPrompt;
	
	    static createFrom(source: any = {}) {
	        return new ConnectResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.host = source["host"];
	        this.name = source["name"];
	        this.sessionId = source["sessionId"];
	        this.hostKeyPrompt = this.convertValues(source["hostKeyPrompt"], HostKeyPrompt);
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
	
	export class SFTPListResult {
	    path: string;
	    entries: sftpclient.Entry[];
	
	    static createFrom(source: any = {}) {
	        return new SFTPListResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.entries = this.convertValues(source["entries"], sftpclient.Entry);
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

export namespace sftpclient {
	
	export class Entry {
	    name: string;
	    path: string;
	    size: number;
	    mode: string;
	    modTime: number;
	    isDir: boolean;
	    owner: string;
	    group: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.mode = source["mode"];
	        this.modTime = source["modTime"];
	        this.isDir = source["isDir"];
	        this.owner = source["owner"];
	        this.group = source["group"];
	    }
	}

}

export namespace sshclient {
	
	export class Stats {
	    host: string;
	    cpuPercent: number;
	    memTotal: number;
	    memUsed: number;
	    memPercent: number;
	    diskTotal: number;
	    diskUsed: number;
	    diskPercent: number;
	    cpuCores: number;
	    cpuThreads: number;
	    procCount: number;
	    loadAvg: string;
	    uptime: string;
	    netRx: number;
	    netTx: number;
	    connCount: number;
	    swapTotal: number;
	    swapUsed: number;
	    swapPercent: number;
	    ioWait: number;
	    kernel: string;
	    boottime: number;
	    fetchedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new Stats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.cpuPercent = source["cpuPercent"];
	        this.memTotal = source["memTotal"];
	        this.memUsed = source["memUsed"];
	        this.memPercent = source["memPercent"];
	        this.diskTotal = source["diskTotal"];
	        this.diskUsed = source["diskUsed"];
	        this.diskPercent = source["diskPercent"];
	        this.cpuCores = source["cpuCores"];
	        this.cpuThreads = source["cpuThreads"];
	        this.procCount = source["procCount"];
	        this.loadAvg = source["loadAvg"];
	        this.uptime = source["uptime"];
	        this.netRx = source["netRx"];
	        this.netTx = source["netTx"];
	        this.connCount = source["connCount"];
	        this.swapTotal = source["swapTotal"];
	        this.swapUsed = source["swapUsed"];
	        this.swapPercent = source["swapPercent"];
	        this.ioWait = source["ioWait"];
	        this.kernel = source["kernel"];
	        this.boottime = source["boottime"];
	        this.fetchedAt = source["fetchedAt"];
	    }
	}

}

export namespace store {
	
	export class CloudQuickCommand {
	    id: number;
	    name: string;
	    command: string;
	    withEnter: boolean;
	    sortOrder: number;
	    category: string;
	
	    static createFrom(source: any = {}) {
	        return new CloudQuickCommand(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.command = source["command"];
	        this.withEnter = source["withEnter"];
	        this.sortOrder = source["sortOrder"];
	        this.category = source["category"];
	    }
	}
	export class CloudSession {
	    id: number;
	    name: string;
	    host: string;
	    port: number;
	    username: string;
	    authType: string;
	    keyPath: string;
	    groupName: string;
	    sortOrder: number;
	    proxyType: string;
	    proxyHost: string;
	    proxyPort: number;
	    proxyUsername: string;
	    execCmd: string;
	    password: string;
	
	    static createFrom(source: any = {}) {
	        return new CloudSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.authType = source["authType"];
	        this.keyPath = source["keyPath"];
	        this.groupName = source["groupName"];
	        this.sortOrder = source["sortOrder"];
	        this.proxyType = source["proxyType"];
	        this.proxyHost = source["proxyHost"];
	        this.proxyPort = source["proxyPort"];
	        this.proxyUsername = source["proxyUsername"];
	        this.execCmd = source["execCmd"];
	        this.password = source["password"];
	    }
	}
	export class CloudPayload {
	    schema: number;
	    sessions: CloudSession[];
	    quickCommands: CloudQuickCommand[];
	
	    static createFrom(source: any = {}) {
	        return new CloudPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema = source["schema"];
	        this.sessions = this.convertValues(source["sessions"], CloudSession);
	        this.quickCommands = this.convertValues(source["quickCommands"], CloudQuickCommand);
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
	
	
	export class QuickCommand {
	    id: number;
	    name: string;
	    command: string;
	    withEnter: boolean;
	    sortOrder: number;
	    category: string;
	
	    static createFrom(source: any = {}) {
	        return new QuickCommand(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.command = source["command"];
	        this.withEnter = source["withEnter"];
	        this.sortOrder = source["sortOrder"];
	        this.category = source["category"];
	    }
	}
	export class Session {
	    id: number;
	    name: string;
	    host: string;
	    port: number;
	    username: string;
	    authType: string;
	    keyPath: string;
	    groupName: string;
	    sortOrder: number;
	    proxyType: string;
	    proxyHost: string;
	    proxyPort: number;
	    proxyUsername: string;
	    execCmd: string;
	    password: string;
	
	    static createFrom(source: any = {}) {
	        return new Session(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.authType = source["authType"];
	        this.keyPath = source["keyPath"];
	        this.groupName = source["groupName"];
	        this.sortOrder = source["sortOrder"];
	        this.proxyType = source["proxyType"];
	        this.proxyHost = source["proxyHost"];
	        this.proxyPort = source["proxyPort"];
	        this.proxyUsername = source["proxyUsername"];
	        this.execCmd = source["execCmd"];
	        this.password = source["password"];
	    }
	}

}

