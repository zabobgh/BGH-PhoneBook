export namespace main {
	
	export class BuildingMeta {
	    building: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new BuildingMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.building = source["building"];
	        this.count = source["count"];
	    }
	}
	export class Entry {
	    id: number;
	    building: string;
	    floor: string;
	    department: string;
	    internal_phone: string;
	    external_phone: string;
	    sort_order: number;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.building = source["building"];
	        this.floor = source["floor"];
	        this.department = source["department"];
	        this.internal_phone = source["internal_phone"];
	        this.external_phone = source["external_phone"];
	        this.sort_order = source["sort_order"];
	    }
	}
	export class Stats {
	    total_entries: number;
	    total_buildings: number;
	    total_floors: number;
	
	    static createFrom(source: any = {}) {
	        return new Stats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total_entries = source["total_entries"];
	        this.total_buildings = source["total_buildings"];
	        this.total_floors = source["total_floors"];
	    }
	}

}

