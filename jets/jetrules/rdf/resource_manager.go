package rdf

// ResourceManager manages all the resources, incl literals and all. Equivalent to RManager in c++

type ResourceManager struct {
	IsLocked      bool
	lastBnodeKey  int
	resourceMap   map[string]*Node
	bnodeMap      map[int]*Node
	literalMap    map[interface{}]*Node
	rootManager   *ResourceManager
	JetsResources *JetResources
}

type JetResources struct {
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

}

func NewResourceManager(rootManager *ResourceManager) *ResourceManager {
	if rootManager != nil {
		rootManager.IsLocked = true
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
	if rm.IsLocked {
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
	if rm.IsLocked {
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
	if rm.IsLocked {
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
	if rm.IsLocked {
		return nil
	}
	r := &Node{Value: data}
	rm.literalMap[data] = r
	return r
}
