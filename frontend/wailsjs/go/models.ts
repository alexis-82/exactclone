export namespace archive {
	
	export class ManifestPartition {
	    name: string;
	    fsType: string;
	    label: string;
	    usedBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new ManifestPartition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.fsType = source["fsType"];
	        this.label = source["label"];
	        this.usedBytes = source["usedBytes"];
	    }
	}
	export class Manifest {
	    version: number;
	    // Go type: time
	    createdAt: any;
	    sourceDisk: string;
	    partitions: ManifestPartition[];
	
	    static createFrom(source: any = {}) {
	        return new Manifest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.sourceDisk = source["sourceDisk"];
	        this.partitions = this.convertValues(source["partitions"], ManifestPartition);
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

export namespace main {
	
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
	export class PartitionEstimate {
	    id: string;
	    usedBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new PartitionEstimate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.usedBytes = source["usedBytes"];
	    }
	}
	export class Estimate {
	    totalBytes: number;
	    partitions: PartitionEstimate[];
	
	    static createFrom(source: any = {}) {
	        return new Estimate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalBytes = source["totalBytes"];
	        this.partitions = this.convertValues(source["partitions"], PartitionEstimate);
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

