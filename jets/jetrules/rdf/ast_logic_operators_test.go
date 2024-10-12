package rdf

import (
	"testing"
)

// This file contains test cases for the Node's logic operators

func TestLogicNOT(t *testing.T) {

	if I(1).NOT().Bool() {
		t.Error("operator failed")
	}
	if !I(0).NOT().Bool() {
		t.Error("operator failed")
	}

	if F(1).NOT().Bool() {
		t.Error("operator failed")
	}
	if !F(0).NOT().Bool() {
		t.Error("operator failed")
	}

	if DD("27/7/1962").NOT().Bool() {
		t.Error("operator failed")
	}
	if DD("27/27/1962").NOT().Bool() {
		t.Error("operator failed")
	}

	if DDT("27/7/1962").NOT().Bool() {
		t.Error("operator failed")
	}
	if DDT("27/27/1962").NOT().Bool() {
		t.Error("operator failed")
	}

	if I(1).NOT().Bool() {
		t.Error("operator failed")
	}
	if !I(0).NOT().Bool() {
		t.Error("operator failed")
	}

	if I(1).NOT().Bool() {
		t.Error("operator failed")
	}
	if !I(0).NOT().Bool() {
		t.Error("operator failed")
	}
}

func TestLogicAND(t *testing.T) {

	if !I(1).AND(I(1)).Bool() {
		t.Error("operator failed")
	}

	if !F(1).AND(I(1)).Bool() {
		t.Error("operator failed")
	}

	if !F(1).AND(DD("27/07/1962")).Bool() {
		t.Error("operator failed")
	}

	if !BN(1).AND(DD("1962/07/27")).Bool() {
		t.Error("operator failed")
	}

	if !R("1").AND(DD("1962/07/27")).Bool() {
		t.Error("operator failed")
	}

	if !R("1").AND(R("1962/07/27")).Bool() {
		t.Error("operator failed")
	}

	if R("1").AND(nil).Bool() {
		t.Error("operator failed")
	}

	if R("1").AND(Null()).Bool() {
		t.Error("operator failed")
	}

	if Null().AND(nil).Bool() {
		t.Error("operator failed")
	}

	if Null().AND(I(1)).Bool() {
		t.Error("operator failed")
	}
}

func TestLogicOR(t *testing.T) {

	if !I(1).OR(I(1)).Bool() {
		t.Error("operator failed")
	}

	if !F(1).OR(I(1)).Bool() {
		t.Error("operator failed")
	}

	if !F(1).OR(DD("27/07/1962")).Bool() {
		t.Error("operator failed")
	}

	if !BN(1).OR(DD("1962/07/27")).Bool() {
		t.Error("operator failed")
	}

	if !R("1").OR(DD("1962/07/27")).Bool() {
		t.Error("operator failed")
	}

	if !R("1").OR(R("1962/07/27")).Bool() {
		t.Error("operator failed")
	}

	if !R("1").OR(nil).Bool() {
		t.Error("operator failed")
	}

	if !R("1").OR(Null()).Bool() {
		t.Error("operator failed")
	}

	if Null().OR(nil).Bool() {
		t.Error("operator failed")
	}

	if Null().OR(Null()).Bool() {
		t.Error("operator failed")
	}

	if !Null().OR(I(1)).Bool() {
		t.Error("operator failed")
	}
}

func TestLogicEQ(t *testing.T) {

	if !I(1).EQ(I(1)).Bool() {
		t.Error("operator failed")
	}

	if !F(1).EQ(I(1)).Bool() {
		t.Error("operator failed")
	}

	if I(10).EQ(I(1)).Bool() {
		t.Error("operator failed")
	}

	if I(10).EQ(F(1)).Bool() {
		t.Error("operator failed")
	}

	if I(10).EQ(Null()).Bool() {
		t.Error("operator failed")
	}

	if F(10).EQ(Null()).Bool() {
		t.Error("operator failed")
	}

	if !F(1).EQ(F(1)).Bool() {
		t.Error("operator failed")
	}

	if F(10).EQ(F(1)).Bool() {
		t.Error("operator failed")
	}

	if F(1).EQ(DD("27/07/1962")).Bool() {
		t.Error("operator failed")
	}

	if F(1).EQ(DDT("27/07/1962")).Bool() {
		t.Error("operator failed")
	}

	if F(1).EQ(R("name")).Bool() {
		t.Error("operator failed")
	}

	if !R("N1").EQ(R("N1")).Bool() {
		t.Error("operator failed")
	}

	if R("N1").EQ(R("N2")).Bool() {
		t.Error("operator failed")
	}

	if R("N1").EQ(BN(1)).Bool() {
		t.Error("operator failed")
	}

	if R("N1").EQ(DD("27/07/1962")).Bool() {
		t.Error("operator failed")
	}

	if R("N1").EQ(DDT("27/07/1962")).Bool() {
		t.Error("operator failed")
	}

	if !DD("27/07/1962").EQ(DD("27/07/1962")).Bool() {
		t.Error("operator failed")
	}

	if DD("27/07/1962").EQ(DD("07/27/1962")).Bool() {
		t.Error("operator failed")
	}

	if DD("27/07/1962").EQ(DDT("27/07/1962")).Bool() {
		t.Error("operator failed")
	}

	if DDT("27/07/1962").EQ(DD("27/07/1962")).Bool() {
		t.Error("operator failed")
	}

	if !Null().EQ(Null()).Bool() {
		t.Error("operator failed")
	}

	if Null().NE(Null()).Bool() {
		t.Error("operator failed")
	}

	if Null().EQ(BN(1)).Bool() {
		t.Error("operator failed")
	}

	if Null().EQ(R("n")).Bool() {
		t.Error("operator failed")
	}

	if Null().EQ(DD("27/07/1962")).Bool() {
		t.Error("operator failed")
	}

	if Null().EQ(DDT("27/07/1962")).Bool() {
		t.Error("operator failed")
	}

}

func TestLogicGE(t *testing.T) {

	if !I(1).GE(I(1)).Bool() {
		t.Error("GE operator failed")
	}

	if !F(1).GE(I(1)).Bool() {
		t.Error("GE operator failed")
	}

	if !I(10).GE(I(1)).Bool() {
		t.Error("GE operator failed")
	}

	if I(1).GE(I(10)).Bool() {
		t.Error("GE operator failed")
	}

	if !I(10).GE(F(1)).Bool() {
		t.Error("GE operator failed")
	}

	if I(10).GE(Null()).Bool() {
		t.Error("GE operator failed")
	}

	if F(10).GE(Null()).Bool() {
		t.Error("GE operator failed")
	}

	if !F(1).GE(F(1)).Bool() {
		t.Error("GE operator failed")
	}

	if !F(10).GE(F(1)).Bool() {
		t.Error("GE operator failed")
	}

	if F(1).GE(F(10)).Bool() {
		t.Error("GE operator failed")
	}

	if F(1).GE(DD("27/07/1962")).Bool() {
		t.Error("GE operator failed")
	}

	if F(1).GE(DDT("27/07/1962")).Bool() {
		t.Error("GE operator failed")
	}

	if F(1).GE(R("name")).Bool() {
		t.Error("GE operator failed")
	}

	if !R("N1").GE(R("N1")).Bool() {
		t.Error("GE operator failed")
	}

	if !R("N2").GE(R("N1")).Bool() {
		t.Error("GE operator failed")
	}

	if R("N1").GE(R("N2")).Bool() {
		t.Error("GE operator failed")
	}

	if R("N1").GE(BN(1)).Bool() {
		t.Error("GE operator failed")
	}

	if R("N1").GE(DD("27/07/1962")).Bool() {
		t.Error("GE operator failed")
	}

	if R("N1").GE(DDT("27/07/1962")).Bool() {
		t.Error("GE operator failed")
	}

	if !DD("27/07/1962").GE(DD("27/07/1962")).Bool() {
		t.Error("GE operator failed")
	}

	if !DD("28/07/1962").GE(DD("27/07/1962")).Bool() {
		t.Error("GE operator failed")
	}

	if DD("26/07/1962").GE(DD("27/07/1962")).Bool() {
		t.Error("GE operator failed")
	}

	if DD("27/07/1962").GE(DDT("27/07/1962")).Bool() {
		t.Error("GE operator failed")
	}

	if DDT("27/07/1962").GE(DD("27/07/1962")).Bool() {
		t.Error("GE operator failed")
	}

	if I(1).GE(Null()).Bool() {
		t.Error("GE operator failed")
	}

	if Null().GE(Null()).Bool() {
		t.Error("GE operator failed")
	}

	if Null().GE(BN(1)).Bool() {
		t.Error("GE operator failed")
	}

	if Null().GE(R("n")).Bool() {
		t.Error("GE operator failed")
	}

	if Null().GE(DD("27/07/1962")).Bool() {
		t.Error("GE operator failed")
	}

	if Null().GE(DDT("27/07/1962")).Bool() {
		t.Error("GE operator failed")
	}

}

func TestLogicGT(t *testing.T) {

	if S("1").GT(S("1")).Bool() {
		t.Error("GT operator failed")
	}

	if S("1").GT(S("2")).Bool() {
		t.Error("GT operator failed")
	}

	if !S("2").GT(S("1")).Bool() {
		t.Error("GT operator failed")
	}

	if I(1).GT(I(1)).Bool() {
		t.Error("GT operator failed")
	}

	if F(1).GT(I(1)).Bool() {
		t.Error("GT operator failed")
	}

	if !I(10).GT(I(1)).Bool() {
		t.Error("GT operator failed")
	}

	if I(1).GT(I(10)).Bool() {
		t.Error("GT operator failed")
	}

	if !I(10).GT(F(1)).Bool() {
		t.Error("GT operator failed")
	}

	if I(10).GT(Null()).Bool() {
		t.Error("GT operator failed")
	}

	if F(10).GT(Null()).Bool() {
		t.Error("GT operator failed")
	}

	if F(1).GT(F(1)).Bool() {
		t.Error("GT operator failed")
	}

	if !F(10).GT(F(1)).Bool() {
		t.Error("GT operator failed")
	}

	if F(1).GT(F(10)).Bool() {
		t.Error("GT operator failed")
	}

	if F(1).GT(DD("27/07/1962")).Bool() {
		t.Error("GT operator failed")
	}

	if F(1).GT(DDT("27/07/1962")).Bool() {
		t.Error("GT operator failed")
	}

	if F(1).GT(R("name")).Bool() {
		t.Error("GT operator failed")
	}

	if R("N1").GT(R("N1")).Bool() {
		t.Error("GT operator failed")
	}

	if !R("N2").GT(R("N1")).Bool() {
		t.Error("GT operator failed")
	}

	if R("N1").GT(R("N2")).Bool() {
		t.Error("GT operator failed")
	}

	if R("N1").GT(BN(1)).Bool() {
		t.Error("GT operator failed")
	}

	if R("N1").GT(DD("27/07/1962")).Bool() {
		t.Error("GT operator failed")
	}

	if R("N1").GT(DDT("27/07/1962")).Bool() {
		t.Error("GT operator failed")
	}

	if DD("27/07/1962").GT(DD("27/07/1962")).Bool() {
		t.Error("GT operator failed")
	}

	if !DD("28/07/1962").GT(DD("27/07/1962")).Bool() {
		t.Error("GT operator failed")
	}

	if DD("26/07/1962").GT(DD("27/07/1962")).Bool() {
		t.Error("GT operator failed")
	}

	if DD("27/07/1962").GT(DDT("27/07/1962")).Bool() {
		t.Error("GT operator failed")
	}

	if DDT("27/07/1962").GT(DD("27/07/1962")).Bool() {
		t.Error("GT operator failed")
	}

	if Null().GT(Null()).Bool() {
		t.Error("GT operator failed")
	}

	if Null().GT(BN(1)).Bool() {
		t.Error("GT operator failed")
	}

	if Null().GT(R("n")).Bool() {
		t.Error("GT operator failed")
	}

	if Null().GT(DD("27/07/1962")).Bool() {
		t.Error("GT operator failed")
	}

	if Null().GT(DDT("27/07/1962")).Bool() {
		t.Error("GT operator failed")
	}
}

func TestLogicLE(t *testing.T) {

	if Null().LE(I(1)).Bool() {
		t.Error("LE operator failed")
	}

	if Null().LE(nil).Bool() {
		t.Error("LE operator failed")
	}

	if Null().LE(Null()).Bool() {
		t.Error("LE operator failed")
	}

	if !I(1).LE(I(1)).Bool() {
		t.Error("LE operator failed")
	}

	if !F(1).LE(I(1)).Bool() {
		t.Error("LE operator failed")
	}

	if I(10).LE(I(1)).Bool() {
		t.Error("LE operator failed")
	}

	if !I(1).LE(I(10)).Bool() {
		t.Error("LE operator failed")
	}

	if I(10).LE(F(1)).Bool() {
		t.Error("LE operator failed")
	}

	if I(10).LE(Null()).Bool() {
		t.Error("LE operator failed")
	}

	if F(10).LE(Null()).Bool() {
		t.Error("LE operator failed")
	}

	if !F(1).LE(F(1)).Bool() {
		t.Error("LE operator failed")
	}

	if F(10).LE(F(1)).Bool() {
		t.Error("LE operator failed")
	}

	if !F(1).LE(F(10)).Bool() {
		t.Error("LE operator failed")
	}

	if F(1).LE(DD("27/07/1962")).Bool() {
		t.Error("LE operator failed")
	}

	if F(1).LE(DDT("27/07/1962")).Bool() {
		t.Error("LE operator failed")
	}

	if F(1).LE(R("name")).Bool() {
		t.Error("LE operator failed")
	}

	if !R("N1").LE(R("N1")).Bool() {
		t.Error("LE operator failed")
	}

	if R("N2").LE(R("N1")).Bool() {
		t.Error("LE operator failed")
	}

	if !R("N1").LE(R("N2")).Bool() {
		t.Error("LE operator failed")
	}

	if R("N1").LE(BN(1)).Bool() {
		t.Error("LE operator failed")
	}

	if R("N1").LE(DD("27/07/1962")).Bool() {
		t.Error("LE operator failed")
	}

	if R("N1").LE(DDT("27/07/1962")).Bool() {
		t.Error("LE operator failed")
	}

	if !DD("27/07/1962").LE(DD("27/07/1962")).Bool() {
		t.Error("LE operator failed")
	}

	if DD("28/07/1962").LE(DD("27/07/1962")).Bool() {
		t.Error("LE operator failed")
	}

	if !DD("26/07/1962").LE(DD("27/07/1962")).Bool() {
		t.Error("LE operator failed")
	}

	if DD("27/07/1962").LE(DDT("27/07/1962")).Bool() {
		t.Error("LE operator failed")
	}

	if DDT("27/07/1962").LE(DD("27/07/1962")).Bool() {
		t.Error("LE operator failed")
	}

	if Null().LE(Null()).Bool() {
		t.Error("LE operator failed")
	}

	if Null().LE(BN(1)).Bool() {
		t.Error("LE operator failed")
	}

	if Null().LE(R("n")).Bool() {
		t.Error("LE operator failed")
	}

	if Null().LE(DD("27/07/1962")).Bool() {
		t.Error("LE operator failed")
	}

	if Null().LE(DDT("27/07/1962")).Bool() {
		t.Error("LE operator failed")
	}

}

func TestLogicLT(t *testing.T) {

	if I(1).LT(I(1)).Bool() {
		t.Error("LT operator failed")
	}

	if F(1).LT(I(1)).Bool() {
		t.Error("LT operator failed")
	}

	if I(10).LT(I(1)).Bool() {
		t.Error("LT operator failed")
	}

	if !I(1).LT(I(10)).Bool() {
		t.Error("LT operator failed")
	}

	if I(10).LT(F(1)).Bool() {
		t.Error("LT operator failed")
	}

	if I(10).LT(Null()).Bool() {
		t.Error("LT operator failed")
	}

	if F(10).LT(Null()).Bool() {
		t.Error("LT operator failed")
	}

	if F(1).LT(F(1)).Bool() {
		t.Error("LT operator failed")
	}

	if F(10).LT(F(1)).Bool() {
		t.Error("LT operator failed")
	}

	if !F(1).LT(F(10)).Bool() {
		t.Error("LT operator failed")
	}

	if F(1).LT(DD("27/07/1962")).Bool() {
		t.Error("LT operator failed")
	}

	if F(1).LT(DDT("27/07/1962")).Bool() {
		t.Error("LT operator failed")
	}

	if F(1).LT(R("name")).Bool() {
		t.Error("LT operator failed")
	}

	if R("N1").LT(R("N1")).Bool() {
		t.Error("LT operator failed")
	}

	if R("N2").LT(R("N1")).Bool() {
		t.Error("LT operator failed")
	}

	if !R("N1").LT(R("N2")).Bool() {
		t.Error("LT operator failed")
	}

	if R("N1").LT(BN(1)).Bool() {
		t.Error("LT operator failed")
	}

	if R("N1").LT(DD("27/07/1962")).Bool() {
		t.Error("LT operator failed")
	}

	if R("N1").LT(DDT("27/07/1962")).Bool() {
		t.Error("LT operator failed")
	}

	if DD("27/07/1962").LT(DD("27/07/1962")).Bool() {
		t.Error("LT operator failed")
	}

	if DD("28/07/1962").LT(DD("27/07/1962")).Bool() {
		t.Error("LT operator failed")
	}

	if !DD("26/07/1962").LT(DD("27/07/1962")).Bool() {
		t.Error("LT operator failed")
	}

	if DD("27/07/1962").LT(DDT("27/07/1962")).Bool() {
		t.Error("LT operator failed")
	}

	if DDT("27/07/1962").LT(DD("27/07/1962")).Bool() {
		t.Error("LT operator failed")
	}

	if Null().LT(Null()).Bool() {
		t.Error("LT operator failed")
	}

	if Null().LT(nil).Bool() {
		t.Error("LT operator failed")
	}

	if Null().LT(BN(1)).Bool() {
		t.Error("LT operator failed")
	}

	if Null().LT(R("n")).Bool() {
		t.Error("LT operator failed")
	}

	if Null().LT(DD("27/07/1962")).Bool() {
		t.Error("LT operator failed")
	}

	if Null().LT(DDT("27/07/1962")).Bool() {
		t.Error("LT operator failed")
	}
}

func TestLogicNE(t *testing.T) {

	if !I(1).NE(nil).Bool() {
		t.Error("NE operator failed")
	}

	if !Null().NE(nil).Bool() {
		t.Error("NE operator failed")
	}

	if I(1).NE(I(1)).Bool() {
		t.Error("NE operator failed")
	}

	if !Null().NE(I(1)).Bool() {
		t.Error("NE operator failed")
	}

	if F(1).NE(I(1)).Bool() {
		t.Error("NE operator failed")
	}

	if !I(10).NE(I(1)).Bool() {
		t.Error("NE operator failed")
	}

	if !I(10).NE(F(1)).Bool() {
		t.Error("NE operator failed")
	}

	if !I(10).NE(Null()).Bool() {
		t.Error("NE operator failed")
	}

	if !F(10).NE(Null()).Bool() {
		t.Error("NE operator failed")
	}

	if F(1).NE(F(1)).Bool() {
		t.Error("NE operator failed")
	}

	if !F(10).NE(F(1)).Bool() {
		t.Error("NE operator failed")
	}

	if !F(1).NE(DD("27/07/1962")).Bool() {
		t.Error("NE operator failed")
	}

	if !F(1).NE(DDT("27/07/1962")).Bool() {
		t.Error("NE operator failed")
	}

	if !F(1).NE(R("name")).Bool() {
		t.Error("NE operator failed")
	}

	if R("N1").NE(R("N1")).Bool() {
		t.Error("NE operator failed")
	}

	if !R("N1").NE(R("N2")).Bool() {
		t.Error("NE operator failed")
	}

	if !R("N1").NE(BN(1)).Bool() {
		t.Error("NE operator failed")
	}

	if BN(1).NE(BN(1)).Bool() {
		t.Error("NE operator failed")
	}

	if !BN(10).NE(BN(1)).Bool() {
		t.Error("NE operator failed")
	}

	if !BN(10).NE(DD("27/07/1962")).Bool() {
		t.Error("NE operator failed")
	}

	if !BN(10).NE(DDT("27/07/1962")).Bool() {
		t.Error("NE operator failed")
	}

	if !R("N1").NE(DD("27/07/1962")).Bool() {
		t.Error("NE operator failed")
	}

	if !R("N1").NE(DDT("27/07/1962")).Bool() {
		t.Error("NE operator failed")
	}

	if DD("27/07/1962").NE(DD("27/07/1962")).Bool() {
		t.Error("NE operator failed")
	}

	if !DD("27/07/1962").NE(DD("07/27/1962")).Bool() {
		t.Error("NE operator failed")
	}

	if !DD("27/07/1962").NE(DDT("27/07/1962")).Bool() {
		t.Error("NE operator failed")
	}

	if !DDT("27/07/1962").NE(DD("27/07/1962")).Bool() {
		t.Error("NE operator failed")
	}

	if Null().NE(Null()).Bool() {
		t.Error("NE operator failed")
	}

	if !Null().NE(BN(1)).Bool() {
		t.Error("NE operator failed")
	}

	if !Null().NE(R("n")).Bool() {
		t.Error("NE operator failed")
	}

	if !Null().NE(DD("27/07/1962")).Bool() {
		t.Error("NE operator failed")
	}

	if !Null().NE(DDT("27/07/1962")).Bool() {
		t.Error("NE operator failed")
	}
}
