package rdf

import "log"

// ResourceManager manages all the resources, incl literals and all. Equivalent to RManager in c++

type ResourceManager struct {
	isLocked      bool
	lastBnodeKey  int
	resourceMap   map[string]*Node
	bnodeMap      map[int]*Node
	literalMap    map[interface{}]*Node
	rootManager   *ResourceManager
	JetsResources *JetResources
}

type JetResources struct {
	jets__client                  *Node
	jets__completed               *Node
	jets__sourcePeriodType        *Node
	jets__currentSourcePeriod     *Node
	jets__currentSourcePeriodDate *Node
	jets__exception               *Node
	jets__input_record            *Node
	jets__istate                  *Node
	jets__key                     *Node
	jets__loop                    *Node
	jets__org                     *Node
	jets__source_period_sequence  *Node
	jets__state                   *Node
	rdf__type                     *Node
}

func NewJetResources(rm *ResourceManager) *JetResources {
	jr := &JetResources{}
	jr.Initialize(rm)
	return jr
}

func (jr *JetResources) Initialize(rm *ResourceManager) {
	if rm == nil {
		return
	}
	// Create the resources
	jr.jets__client = rm.NewResource("jets:client")
	jr.jets__completed = rm.NewResource("jets:completed")
	jr.jets__sourcePeriodType = rm.NewResource("jets:sourcePeriodType")
	jr.jets__currentSourcePeriod = rm.NewResource("jets:currentSourcePeriod")
	jr.jets__currentSourcePeriodDate = rm.NewResource("jets:currentSourcePeriodDate")
	jr.jets__exception = rm.NewResource("jets:exception")
	jr.jets__input_record = rm.NewResource("jets:input_record")
	jr.jets__istate = rm.NewResource("jets:istate")
	jr.jets__key = rm.NewResource("jets:key")
	jr.jets__loop = rm.NewResource("jets:loop")
	jr.jets__org = rm.NewResource("jets:org")
	jr.jets__source_period_sequence = rm.NewResource("jets:source_period_sequence")
	jr.jets__state = rm.NewResource("jets:state")
	jr.rdf__type = rm.NewResource("rdf:type")
}

func NewResourceManager(rootManager *ResourceManager) *ResourceManager {
	if rootManager != nil {
		rootManager.isLocked = true
	}
	rm := &ResourceManager{
		resourceMap: make(map[string]*Node, 100),
		bnodeMap:    make(map[int]*Node, 50),
		literalMap:  make(map[interface{}]*Node, 200),
		rootManager: rootManager,
	}
	rm.JetsResources = NewJetResources(rm)
	return rm
}

func (rm *ResourceManager) GetResource(name string) *Node {
	if rm.rootManager != nil {
		n := rm.rootManager.resourceMap[name]
		if n != nil {
			return n
		}
	}
	return rm.resourceMap[name]
}

func (rm *ResourceManager) NewResource(name string) *Node {
	v := rm.GetResource(name)
	if v != nil {
		return v
	}
	if rm.isLocked {
		log.Println("error: NewResource called when ResourceManger is locked")
		return nil
	}
	r := R(name)
	rm.resourceMap[name] = r
	return r
}

func (rm *ResourceManager) GetBNode(key int) *Node {
	if rm.rootManager != nil {
		n := rm.rootManager.bnodeMap[key]
		if n != nil {
			return n
		}
	}
	return rm.bnodeMap[key]
}

func (rm *ResourceManager) NewBNode() *Node {
	if rm.isLocked {
		log.Println("error: NewBNode called when ResourceManger is locked")
		return nil
	}
	r := BN(rm.lastBnodeKey)
	rm.bnodeMap[rm.lastBnodeKey] = r
	rm.lastBnodeKey += 1
	return r
}

func (rm *ResourceManager) CreateBNode(key int) *Node {
	v := rm.GetBNode(key)
	if v != nil {
		return v
	}
	if rm.isLocked {
		log.Println("error: CreateBNode called when ResourceManger is locked")
		return nil
	}
	r := BN(key)
	rm.bnodeMap[rm.lastBnodeKey] = r
	return r
}

func (rm *ResourceManager) GetLiteral(data interface{}) *Node {
	if rm.rootManager != nil {
		n := rm.rootManager.literalMap[data]
		if n != nil {
			return n
		}
	}
	return rm.literalMap[data]
}

func (rm *ResourceManager) NewLiteral(data interface{}) *Node {
	v := rm.GetLiteral(data)
	if v != nil {
		return v
	}
	if rm.isLocked {
		log.Println("error: NewLiteral called when ResourceManger is locked")
		return nil
	}
	r := &Node{Value: data}
	rm.literalMap[data] = r
	return r
}
