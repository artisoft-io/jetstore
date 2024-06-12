package rdf

// ResourceManager manages all the resources, incl literals and all. Equivalent to RManager in c++

// bool          is_locked_;
// int           last_bnode_key_;
// DataMap       lmap_;
// RManagerPtr   root_mgr_p_;
// JetsResources jets_resources_;

type ResourceManager struct {
	IsLocked bool
	lastBnodeKey int
	resourceMap map[string]*Node
	bnodeMap map[int]*Node
	literalMap map[interface{}]*Node
	rootManager *ResourceManager
	JetsResources *JetResources
}

type JetResources struct {

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
	r := BN(rm.lastBnodeKey)
	rm.bnodeMap[rm.lastBnodeKey] = r
	rm.lastBnodeKey += 1
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
	r := &Node{Value: data}
	rm.literalMap[data] = r
	return r
}


// template<class T>
// inline r_index 
// get_literal(T v) const
// {
// 	Rptr lptr = mkLiteral(v);
// 	return get_item(lptr);
// }
