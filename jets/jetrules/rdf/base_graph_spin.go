package rdf

 // Map (u, v, w) ==> (s, p, o) according to spin code.
 //
 //  - (u, v, w) => 's' => (u, v, w) <=> (s, p, o)
 //  - (u, v, w) => 'p' => (u, v, w) <=> (p, o, s)
 //  - (u, v, w) => 'o' => (u, v, w) <=> (o, s, p)
 //
// From c++ implementation:
//  lookup_uvw2spo(char const spin, 
// 	 r_index  const& u, r_index  const& v, r_index  const& w, 
// 	 r_index  &s, r_index  &p, r_index  &o)
//  {
// 	 if(spin == 's') {					// case 'spo'  <==> "uvw'
// 		 s = u;
// 		 p = v;
// 		 o = w;
// 	 } else if(spin == 'p') {	// case 'pos'  <==> "uvw'
// 		 s = w;
// 		 p = u;
// 		 o = v;
// 	 } else {									// case 'osp'  <==> "uvw'
// 		 s = v;
// 		 p = w;
// 		 o = u;
// 	 }
//  }
func mapUVW2SPO(spin byte, u, v, w *Node) (*Node,*Node,*Node) {
	switch spin {
	case 's':
		// case 'spo'  <==> "uvw'
		return u, v, w
	case 'p':
		// case 'pos'  <==> "uvw'
		return  w, u, v
	default:
		// case 'osp'  <==> "uvw'
		return v, w, u
	}
}
func mapUVW2SPOArr(spin byte, u, v, w *Node) [3]*Node {
	switch spin {
	case 's':
		// case 'spo'  <==> "uvw'
		return [3]*Node{u, v, w}
	case 'p':
		// case 'pos'  <==> "uvw'
		return  [3]*Node{w, u, v}
	default:
		// case 'osp'  <==> "uvw'
		return [3]*Node{v, w, u}
	}
}
 
// Map (s, p, o) ==> (u, v, w) according to spin code.
//
//  - (s, p, o) => 's' => (s, p, o) <=> (u, v, w)
//  - (s, p, o) => 'p' => (p, o, s) <=> (u, v, w)
//  - (s, p, o) => 'o' => (o, s, p) <=> (u, v, w)
//
// From c++ implementation:
//  lookup_spo2uvw(char const spin, 
// 	 r_index  const& s, r_index  const& p, r_index  const& o, 
// 	 r_index  &u, r_index  &v, r_index  &w)
//  {
// 	 if(spin == 's') {					// case 'spo'  <==> "uvw'
// 		 u = s;
// 		 v = p;
// 		 w = o;
// 	 } else if(spin == 'p') {	// case 'pos'  <==> "uvw'
// 		 w = s;
// 		 u = p;
// 		 v = o;
// 	 } else {									// case 'osp'  <==> "uvw'
// 		 v = s;
// 		 w = p;
// 		 u = o;
// 	 }
//  }
func mapSPO2UVW(spin byte, s, p, o *Node) (*Node,*Node,*Node) {
	switch spin {
	case 's':
		// case 'spo'  <==> "uvw'
		return s, p, o
	case 'p':
		// case 'pos'  <==> "uvw'
		return  p, o, s
	default:
		// case 'osp'  <==> "uvw'
		return o, s, p
	}
}
func mapSPO2UVWArr(spin byte, s, p, o *Node) [3]*Node {
	switch spin {
	case 's':
		// case 'spo'  <==> "uvw'
		return [3]*Node{s, p, o}
	case 'p':
		// case 'pos'  <==> "uvw'
		return  [3]*Node{p, o, s}
	default:
		// case 'osp'  <==> "uvw'
		return [3]*Node{o, s, p}
	}
}
 