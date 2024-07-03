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
	Jets__sourcePeriodType        *Node
	Jets__currentSourcePeriod     *Node
	Jets__currentSourcePeriodDate *Node
	Jets__client                  *Node
	Jets__completed               *Node
	Jets__entity_property         *Node
	Jets__exception               *Node
	Jets__from                    *Node
	Jets__input_record            *Node
	Jets__istate                  *Node
	Jets__key                     *Node
	Jets__length                  *Node
	Jets__lookup_multi_rows       *Node
	Jets__lookup_row              *Node
	Jets__loop                    *Node
	Jets__max_vertex_visits       *Node
	Jets__operator                *Node
	Jets__org                     *Node
	Jets__range_value             *Node
	Jets__replace_chars           *Node
	Jets__replace_with            *Node
	Jets__source_period_sequence  *Node
	Jets__state                   *Node
	Jets__value_property          *Node
	Rdf__type                     *Node
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
	jr.Jets__client = rm.NewResource("jets:client")
	jr.Jets__completed = rm.NewResource("jets:completed")
	jr.Jets__currentSourcePeriod = rm.NewResource("jets:currentSourcePeriod")
	jr.Jets__currentSourcePeriodDate = rm.NewResource("jets:currentSourcePeriodDate")
	jr.Jets__entity_property = rm.NewResource("jets:entity_property")
	jr.Jets__exception = rm.NewResource("jets:exception")
	jr.Jets__from = rm.NewResource("jets:from")
	jr.Jets__input_record = rm.NewResource("jets:InputRecord")
	jr.Jets__istate = rm.NewResource("jets:iState")
	jr.Jets__key = rm.NewResource("jets:key")
	jr.Jets__length = rm.NewResource("jets:length")
	jr.Jets__lookup_multi_rows = rm.NewResource("jets:lookup_multi_rows")
	jr.Jets__lookup_row = rm.NewResource("jets:lookup_row")
	jr.Jets__loop = rm.NewResource("jets:loop")
	jr.Jets__max_vertex_visits = rm.NewResource("jets:max_vertex_visits")
	jr.Jets__operator = rm.NewResource("jets:operator")
	jr.Jets__org = rm.NewResource("jets:org")
	jr.Jets__range_value = rm.NewResource("jets:range_value")
	jr.Jets__replace_chars = rm.NewResource("jets:replace_chars")
	jr.Jets__replace_with = rm.NewResource("jets:replace_with")
	jr.Jets__source_period_sequence = rm.NewResource("jets:source_period_sequence")
	jr.Jets__sourcePeriodType = rm.NewResource("jets:sourcePeriodType")
	jr.Jets__state = rm.NewResource("jets:State")
	jr.Jets__value_property = rm.NewResource("jets:value_property")
	jr.Rdf__type = rm.NewResource("rdf:type")
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
	if rootManager != nil {
		rm.JetsResources = rootManager.JetsResources
	} else {
		rm.JetsResources = NewJetResources(rm)
	}
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
