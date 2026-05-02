export namespace api {
	
	export class VMPEngine {
	
	
	    static createFrom(source: any = {}) {
	        return new VMPEngine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

