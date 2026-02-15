export namespace main {
	
	export class DownloadHistory {
	    id: string;
	    url: string;
	    title: string;
	    type: string;
	    resolution: string;
	    audioFormat: string;
	    filePath: string;
	    fileSize: number;
	    // Go type: time
	    downloadedAt: any;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadHistory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.url = source["url"];
	        this.title = source["title"];
	        this.type = source["type"];
	        this.resolution = source["resolution"];
	        this.audioFormat = source["audioFormat"];
	        this.filePath = source["filePath"];
	        this.fileSize = source["fileSize"];
	        this.downloadedAt = this.convertValues(source["downloadedAt"], null);
	        this.status = source["status"];
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
	export class DownloadTask {
	    url: string;
	    title: string;
	    type: string;
	    resolution: string;
	    cleanRes: string;
	    audioFormat: string;
	    audioQuality: string;
	    videoCodec: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadTask(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.title = source["title"];
	        this.type = source["type"];
	        this.resolution = source["resolution"];
	        this.cleanRes = source["cleanRes"];
	        this.audioFormat = source["audioFormat"];
	        this.audioQuality = source["audioQuality"];
	        this.videoCodec = source["videoCodec"];
	    }
	}
	export class VideoInfo {
	    title: string;
	    duration: number;
	    resolutions: string[];
	
	    static createFrom(source: any = {}) {
	        return new VideoInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.duration = source["duration"];
	        this.resolutions = source["resolutions"];
	    }
	}

}

