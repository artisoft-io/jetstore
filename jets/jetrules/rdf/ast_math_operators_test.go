package rdf

import (
	"testing"
)

// This file contains test cases for the Node's logic operators

func TestMathABS(t *testing.T) {

	if !I(1).ABS().EQ(I(1)).Bool() {
		t.Error("operator failed")
	}
	if !I(-1).ABS().EQ(I(1)).Bool() {
		t.Error("operator failed")
	}
	if I(-1).ABS().Value.(int) != 1 {
		t.Error("operator failed")
	}

	if !F(1).ABS().EQ(I(1)).Bool() {
		t.Error("operator failed")
	}
	if !F(-1).ABS().EQ(I(1)).Bool() {
		t.Error("operator failed")
	}
	if F(-1).ABS().Value.(float64) != 1 {
		t.Error("operator failed")
	}

	if DD("1960/10/10").ABS() != nil {
		t.Error("operator failed")
	}

	if DDT("1960/10/10").ABS() != nil {
		t.Error("operator failed")
	}

	if Null().ABS() != nil {
		t.Error("operator failed")
	}
}

func TestMathADD(t *testing.T) {
	if !I(1).ADD(I(1)).EQ(I(2)).Bool() {
		t.Error("operator failed")
	}
	if I(1).ADD(I(2)).EQ(I(2)).Bool() {
		t.Error("operator failed")
	}
	
	if !F(1).ADD(I(1)).EQ(I(2)).Bool() {
		t.Error("operator failed")
	}
	if !F(1).ADD(F(1)).EQ(F(2)).Bool() {
		t.Error("operator failed")
	}
	if F(1).ADD(I(2)).EQ(I(2)).Bool() {
		t.Error("operator failed")
	}

	if !DD("10/10/2010").ADD(I(1)).EQ(DD("10/11/2010")).Bool() {
		t.Error("operator failed")
	}

	if !DD("10/10/2010").ADD(F(1)).EQ(DD("10/11/2010")).Bool() {
		t.Error("operator failed")
	}

	if !DDT("10/10/2010").ADD(I(1)).EQ(DDT("10/11/2010")).Bool() {
		t.Error("operator failed")
	}

	if DD("10/10/2010").ADD(DD("10/11/2010")) != nil {
		t.Error("operator failed")
	}

	if DDT("10/10/2010").ADD(DDT("10/11/2010")) != nil {
		t.Error("operator failed")
	}

	if !S("Hello ").ADD(S("World")).EQ(S("Hello World")).Bool() {
		t.Error("operator failed")
	}

	if !S("Hello ").ADD(I(1)).EQ(S("Hello 1")).Bool() {
		t.Error("operator failed")
	}

	if !S("Hello ").ADD(F(1)).EQ(S("Hello 1")).Bool() {
		t.Error("operator failed")
	}
	if !S("Hello ").ADD(F(1.5)).EQ(S("Hello 1.5")).Bool() {
		t.Error("operator failed")
	}
}

func TestMathDIV(t *testing.T) {
	if !I(1).DIV(I(1)).EQ(I(1)).Bool() {
		t.Error("operator failed")
	}
	if !I(4).DIV(I(2)).EQ(I(2)).Bool() {
		t.Error("operator failed")
	}

	if !F(1).DIV(F(1)).EQ(F(1)).Bool() {
		t.Error("operator failed")
	}
	if !F(4).DIV(F(2)).EQ(F(2)).Bool() {
		t.Error("operator failed")
	}

	if I(1).DIV(S("1")) != nil {
		t.Error("operator failed")
	}
	if F(1).DIV(S("1")) != nil {
		t.Error("operator failed")
	}
}

func TestMathMUL(t *testing.T) {
	if !I(1).MUL(I(1)).EQ(I(1)).Bool() {
		t.Error("operator failed")
	}
	if !I(2).MUL(I(2)).EQ(I(4)).Bool() {
		t.Error("operator failed")
	}
	if !F(2).MUL(F(2)).EQ(F(4)).Bool() {
		t.Error("operator failed")
	}
	if !F(2).MUL(I(2)).EQ(F(4)).Bool() {
		t.Error("operator failed")
	}

	if S("1").MUL(S("2")) != nil {
		t.Error("operator failed")
	}

	if !S("1").MUL(I(2)).EQ(S("11")).Bool() {
		t.Error("operator failed")
	}
	if !S("F1").MUL(I(2)).EQ(S("F1F1")).Bool() {
		t.Error("operator failed")
	}
	if !S("F1").MUL(F(2)).EQ(S("F1F1")).Bool() {
		t.Error("operator failed")
	}
}

func TestMathSUB(t *testing.T) {
	if !I(1).SUB(I(1)).EQ(I(0)).Bool() {
		t.Error("operator failed")
	}

	if !F(1).SUB(I(1)).EQ(I(0)).Bool() {
		t.Error("operator failed")
	}
	if !F(1).SUB(F(1)).EQ(F(0)).Bool() {
		t.Error("operator failed")
	}

	if !S("S1TT").SUB(S("TT")).EQ(S("S1")).Bool() {
		t.Error("operator failed")
	}

	if S("S1TT").SUB(S("TTT")).EQ(S("S1")).Bool() {
		t.Error("operator failed")
	}

	if S("S1TT").SUB(DD("01/01/2000")) != nil {
		t.Error("operator failed")
	}

	if !DD("07/27/2000").SUB(DD("07/20/2000")).EQ(I(7)).Bool() {
		t.Error("operator failed")
	}
	if !DD("2000/7/27").SUB(DD("07/20/2000")).EQ(I(7)).Bool() {
		t.Error("operator failed")
	}
	if !DDT("2000/7/27").SUB(DDT("07/20/2000")).EQ(I(7)).Bool() {
		t.Error("operator failed")
	}

	if !DD("07/20/2000").SUB(DD("07/27/2000")).EQ(I(-7)).Bool() {
		t.Error("operator failed")
	}
}
