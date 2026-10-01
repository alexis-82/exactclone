export namespace disk {
	
	export class Partition {
	    id: string;
	    path: string;
	    number: number;
	    sizeBytes: number;
	    fsType: string;
	    label: string;
	    mountPoints: string[];
	    supported: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Partition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.number = source["number"];
	        this.sizeBytes = source["sizeBytes"];
	        this.fsType = source["fsType"];
	        this.label = source["label"];
	        this.mountPoints = source["mountPoints"];
	        this.supported = source["supported"];
	    }
	}
	export class Disk {
	    id: string;
	    path: string;
	    model: string;
	    serial: string;
	    bus: string;
	    sizeBytes: number;
	    removable: boolean;
	    isSystem: boolean;
	    partitions: Partition[];
	
	    static createFrom(source: any = {}) {
	        return new Disk(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.model = source["model"];
	        this.serial = source["serial"];
	        this.bus = source["bus"];
	        this.sizeBytes = source["sizeBytes"];
	        this.removable = source["removable"];
	        this.isSystem = source["isSystem"];
	        this.partitions = this.convertValues(source["partitions"], Partition);
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

export namespace image {
	
	export class Info {
	    format: string;
	    version: number;
	    sourceDisk: string;
	    sizeBytes: number;
	    // Go type: time
	    createdAt: any;
	    sha256?: string;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.version = source["version"];
	        this.sourceDisk = source["sourceDisk"];
	        this.sizeBytes = source["sizeBytes"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.sha256 = source["sha256"];
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

export namespace main {
	
	export class AppInfo {
	    name: string;
	    version: string;
	    author: string;
	    os: string;
	    arch: string;
	    goVersion: string;
	    wailsVersion: string;
	    configPath: string;
	    elevated: boolean;
	    devSafe: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.author = source["author"];
	        this.os = source["os"];
	        this.arch = source["arch"];
	        this.goVersion = source["goVersion"];
	        this.wailsVersion = source["wailsVersion"];
	        this.configPath = source["configPath"];
	        this.elevated = source["elevated"];
	        this.devSafe = source["devSafe"];
	    }
	}
	export class Config {
	    language: string;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language = source["language"];
	    }
	}

}

export namespace mount {
	
	export class Info {
	    usedBytes: number;
	    freeBytes: number;
	    fsType: string;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.usedBytes = source["usedBytes"];
	        this.freeBytes = source["freeBytes"];
	        this.fsType = source["fsType"];
	    }
	}

}

