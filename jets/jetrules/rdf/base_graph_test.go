package rdf

import (
	"fmt"
	"testing"
)

// This file contains test cases for the BaseGraph in rdf package
func TestBaseGraph(t *testing.T) {

	// test BaseGraph type
	rm := NewResourceManager(nil)
	if rm == nil {
		t.Errorf("error: nil returned by NewResourceManager")
	}
	s := rm.NewResource("s")
	p := rm.NewResource("p")
	o := rm.NewResource("o")
	bg := NewBaseGraph("META", 's')
	if bg.Size() != 0 {
		t.Error("NewBaseGraph size != 0")
	}
	bg.Insert(s, p, o)
	if bg.Size() != 1 {
		t.Errorf("NewBaseGraph size != 1, it's %d", bg.Size())
	}
	if !bg.Contains(s, p, o) {
		t.Error("bg.Contains(s, p, o) fails")
	}
	bg.Insert(s, p, o)
	if bg.Size() != 1 {
		t.Errorf("Insert again (s,p,o) size != 1, it's %d", bg.Size())
	}
	u := rm.NewResource("u")
	v := rm.NewResource("v")
	w := rm.NewResource("w")
	bg.Insert(u, v, w)
	if bg.Size() != 2 {
		t.Errorf("Insert (u,v,w) size != 2, it's %d", bg.Size())
	}
}

func TestBaseGraphRetract(t *testing.T) {

	// test BaseGraph type
	rm := NewResourceManager(nil)
	if rm == nil {
		t.Errorf("error: nil returned by NewResourceManager")
	}
	s := rm.NewResource("s")
	p := rm.NewResource("p")
	o := rm.NewResource("o")
	bg := NewBaseGraph("META", 's')
	if bg.Size() != 0 {
		t.Error("NewBaseGraph size != 0")
	}
	bg.Insert(s, p, o)
	if bg.Size() != 1 {
		t.Errorf("NewBaseGraph size != 1, it's %d", bg.Size())
	}
	if bg.GetRefCount(s, p, o) != 1 {
		t.Errorf("NewBaseGraph ref count != 1, it's %d", bg.GetRefCount(s, p, o))
	}
	if !bg.Contains(s, p, o) {
		t.Error("bg.Contains(s, p, o) fails")
	}
	bg.Insert(s, p, o)
	if bg.Size() != 1 {
		t.Errorf("Insert again (s,p,o) size != 1, it's %d", bg.Size())
	}
	if bg.GetRefCount(s, p, o) != 2 {
		t.Errorf("NewBaseGraph ref count != 2, it's %d", bg.GetRefCount(s, p, o))
	}
	deleted := bg.Retract(s, p, o)
	if deleted {
		t.Errorf("NewBaseGraph retract unexpected deleted triple")
	}
	if bg.GetRefCount(s, p, o) != 1 {
		t.Errorf("NewBaseGraph ref count != 1 after retract, it's %d", bg.GetRefCount(s, p, o))
	}
	deleted = bg.Retract(s, p, o)
	if !deleted {
		t.Errorf("NewBaseGraph retract did not delete triple")
	}

	u := rm.NewResource("u")
	v := rm.NewResource("v")
	w := rm.NewResource("w")
	bg.Insert(u, v, w)
	bg.Insert(u, v, w)
	if bg.Size() != 1 {
		t.Errorf("Insert (u,v,w) size != 1, it's %d", bg.Size())
	}
	if bg.GetRefCount(u, v, w) != 2 {
		t.Errorf("NewBaseGraph ref count expected to be 2, it's %d", bg.GetRefCount(u, v, w))
	}
	deleted = bg.Erase(u, v, w)
	if !deleted {
		t.Errorf("NewBaseGraph erase did not delete triple")
	}
	if bg.GetRefCount(u, v, w) != 0 {
		t.Errorf("NewBaseGraph ref count expected to be 0, it's %d", bg.GetRefCount(u, v, w))
	}
}

func TestBaseGraphIterator(t *testing.T) {
	rm := NewResourceManager(nil)
	s := rm.NewResource("s")
	p := rm.NewResource("p")
	o := rm.NewResource("o")
	bg := NewBaseGraph("META", 's')
	bg.Insert(s, p, o)
	u := rm.NewResource("u")
	v := rm.NewResource("v")
	w := rm.NewResource("w")
	bg.Insert(u, v, w)
	one := rm.NewLiteral(1)
	two := rm.NewLiteral(2)
	bg.Insert(s, p, one)
	bg.Insert(s, p, two)

	itor := bg.Find()
	if itor == nil {
		t.Errorf("while calling Find() got nil itor")
	} else {
		defer itor.Done()
		for t3 := range itor.Itor {
			fmt.Printf("Got (%s, %s, %s)\n", t3[0], t3[1], t3[2])
		}	
	}
}

func TestBenchBaseGraph(t *testing.T) {
	rm := NewResourceManager(nil)
	bg := NewBaseGraph("META", 's')
	for i := 0; i < 100; i++ {
		s := rm.NewResource(fmt.Sprintf("subject%d", i))
		for j := 0; j < 20; j++ {
			p := rm.NewResource(fmt.Sprintf("predicate%d", j))
			o1 := rm.NewLiteral(i + j)
			o2 := rm.NewLiteral(fmt.Sprintf("obj%d", j))
			bg.Insert(s, p, o1)
			bg.Insert(s, p, o2)
		}
	}

	fmt.Println("The graph contains",bg.Size(),"triples")

	for i := 0; i < 100; i += 10 {
		s := rm.NewResource(fmt.Sprintf("subject%d", i))
		itor := bg.FindU(s)
		if itor == nil {
			t.Errorf("while FindU got nil itor")
		} else {
			for t3 := range itor.Itor {
				if s != t3[0] {
					t.Error("s != t3[0]")
				}
			}
			itor.Done()	
		}
		for j := 0; j < 20; j += 5 {
			p := rm.NewResource(fmt.Sprintf("predicate%d", j))
			itor := bg.FindUV(s, p)
			if itor == nil {
				t.Errorf("while FindUV got nil itor")
			} else {
				for t3 := range itor.Itor {
					if s != t3[0] {
						t.Error("s != t3[0]")
					}
					if p != t3[1] {
						t.Error("p != t3[1]")
					}
					fmt.Printf("Got (%s, %s, %s)\n", t3[0], t3[1], t3[2])
				}	
			}
		}
	}
}
