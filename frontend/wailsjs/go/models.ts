export namespace torrentclient {
	
	export class peerDTO {
	    peerID: string;
	    ip: string;
	    port: number;
	
	    static createFrom(source: any = {}) {
	        return new peerDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.peerID = source["peerID"];
	        this.ip = source["ip"];
	        this.port = source["port"];
	    }
	}
	export class ConnectionDataStatusDTO {
	    downloadedData: number;
	    uploadedData: number;
	    downloadSpeed: number;
	    uploadSpeed: number;
	    peerDTO: peerDTO;
	    peerBitfield: number[];
	
	    static createFrom(source: any = {}) {
	        return new ConnectionDataStatusDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.downloadedData = source["downloadedData"];
	        this.uploadedData = source["uploadedData"];
	        this.downloadSpeed = source["downloadSpeed"];
	        this.uploadSpeed = source["uploadSpeed"];
	        this.peerDTO = this.convertValues(source["peerDTO"], peerDTO);
	        this.peerBitfield = source["peerBitfield"];
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
	export class TorrentStatsDTO {
	    uploadedBytes: number;
	    downloadedBytes: number;
	    connectionDataStatuses: {[key: string]: ConnectionDataStatusDTO};
	    piecesOnDiskCount: number;
	    piecesStoredToDisk: number[];
	    requestedPieces: number[];
	
	    static createFrom(source: any = {}) {
	        return new TorrentStatsDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uploadedBytes = source["uploadedBytes"];
	        this.downloadedBytes = source["downloadedBytes"];
	        this.connectionDataStatuses = this.convertValues(source["connectionDataStatuses"], ConnectionDataStatusDTO, true);
	        this.piecesOnDiskCount = source["piecesOnDiskCount"];
	        this.piecesStoredToDisk = source["piecesStoredToDisk"];
	        this.requestedPieces = source["requestedPieces"];
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
	export class FileDataDTO {
	    fileLength: number;
	    filePath: string;
	
	    static createFrom(source: any = {}) {
	        return new FileDataDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fileLength = source["fileLength"];
	        this.filePath = source["filePath"];
	    }
	}
	export class TorrentDataDTO {
	    Announce: string;
	    createdBy: string;
	    creationDate: number;
	    encoding: string;
	    comment: string;
	    pieceLength: number;
	    pieceNumber: number;
	    private: boolean;
	    name: string;
	    fileLength: number;
	    files: FileDataDTO[];
	    torrentSize: number;
	    infohash: string;
	
	    static createFrom(source: any = {}) {
	        return new TorrentDataDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Announce = source["Announce"];
	        this.createdBy = source["createdBy"];
	        this.creationDate = source["creationDate"];
	        this.encoding = source["encoding"];
	        this.comment = source["comment"];
	        this.pieceLength = source["pieceLength"];
	        this.pieceNumber = source["pieceNumber"];
	        this.private = source["private"];
	        this.name = source["name"];
	        this.fileLength = source["fileLength"];
	        this.files = this.convertValues(source["files"], FileDataDTO);
	        this.torrentSize = source["torrentSize"];
	        this.infohash = source["infohash"];
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
	export class TorrentDTO {
	    torrentData: TorrentDataDTO;
	    torrentStats: TorrentStatsDTO;
	
	    static createFrom(source: any = {}) {
	        return new TorrentDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.torrentData = this.convertValues(source["torrentData"], TorrentDataDTO);
	        this.torrentStats = this.convertValues(source["torrentStats"], TorrentStatsDTO);
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
	export class AggregateReport {
	    torrents: TorrentDTO[];
	
	    static createFrom(source: any = {}) {
	        return new AggregateReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.torrents = this.convertValues(source["torrents"], TorrentDTO);
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

