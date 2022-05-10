# Generated from JetRule.g4 by ANTLR 4.10.1
# encoding: utf-8
from antlr4 import *
from io import StringIO
import sys
if sys.version_info[1] > 5:
	from typing import TextIO
else:
	from typing.io import TextIO

def serializedATN():
    return [
        4,1,66,732,2,0,7,0,2,1,7,1,2,2,7,2,2,3,7,3,2,4,7,4,2,5,7,5,2,6,7,
        6,2,7,7,7,2,8,7,8,2,9,7,9,2,10,7,10,2,11,7,11,2,12,7,12,2,13,7,13,
        2,14,7,14,2,15,7,15,2,16,7,16,2,17,7,17,2,18,7,18,2,19,7,19,2,20,
        7,20,2,21,7,21,2,22,7,22,2,23,7,23,2,24,7,24,2,25,7,25,2,26,7,26,
        2,27,7,27,2,28,7,28,2,29,7,29,2,30,7,30,2,31,7,31,2,32,7,32,2,33,
        7,33,2,34,7,34,2,35,7,35,2,36,7,36,2,37,7,37,2,38,7,38,2,39,7,39,
        2,40,7,40,2,41,7,41,2,42,7,42,2,43,7,43,2,44,7,44,2,45,7,45,2,46,
        7,46,2,47,7,47,2,48,7,48,1,0,5,0,100,8,0,10,0,12,0,103,9,0,1,0,1,
        0,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,3,1,117,8,1,1,2,1,2,1,
        2,1,2,1,2,1,2,1,3,1,3,1,3,5,3,128,8,3,10,3,12,3,131,9,3,1,3,1,3,
        5,3,135,8,3,10,3,12,3,138,9,3,1,3,1,3,1,3,1,4,1,4,1,4,5,4,146,8,
        4,10,4,12,4,149,9,4,1,4,5,4,152,8,4,10,4,12,4,155,9,4,1,5,1,5,1,
        5,1,5,1,5,1,5,3,5,163,8,5,1,6,1,6,1,6,1,6,5,6,169,8,6,10,6,12,6,
        172,9,6,1,6,1,6,1,6,1,6,5,6,178,8,6,10,6,12,6,181,9,6,1,6,1,6,5,
        6,185,8,6,10,6,12,6,188,9,6,1,6,1,6,1,6,5,6,193,8,6,10,6,12,6,196,
        9,6,1,6,1,6,1,6,1,6,5,6,202,8,6,10,6,12,6,205,9,6,1,6,1,6,5,6,209,
        8,6,10,6,12,6,212,9,6,1,6,1,6,3,6,216,8,6,1,6,5,6,219,8,6,10,6,12,
        6,222,9,6,1,6,1,6,1,6,1,7,1,7,1,7,5,7,230,8,7,10,7,12,7,233,9,7,
        1,7,5,7,236,8,7,10,7,12,7,239,9,7,1,8,1,8,1,8,3,8,244,8,8,1,8,1,
        8,1,8,5,8,249,8,8,10,8,12,8,252,9,8,1,8,5,8,255,8,8,10,8,12,8,258,
        9,8,1,9,1,9,5,9,262,8,9,10,9,12,9,265,9,9,1,9,1,9,1,9,1,9,1,10,1,
        10,1,11,1,11,1,12,1,12,1,12,1,12,5,12,279,8,12,10,12,12,12,282,9,
        12,1,12,1,12,1,12,1,12,5,12,288,8,12,10,12,12,12,291,9,12,1,12,1,
        12,5,12,295,8,12,10,12,12,12,298,9,12,1,12,1,12,3,12,302,8,12,1,
        12,5,12,305,8,12,10,12,12,12,308,9,12,1,12,1,12,1,12,1,13,1,13,1,
        13,5,13,316,8,13,10,13,12,13,319,9,13,1,13,5,13,322,8,13,10,13,12,
        13,325,9,13,1,14,1,14,1,14,1,14,1,14,1,14,1,14,1,14,1,14,3,14,336,
        8,14,1,15,1,15,1,15,1,15,1,15,1,15,1,16,1,16,1,16,1,16,1,16,1,16,
        1,17,1,17,1,17,1,17,1,17,1,17,1,18,1,18,1,18,1,18,1,18,1,18,1,19,
        1,19,1,19,1,19,1,19,1,19,1,20,1,20,1,20,1,20,1,20,1,20,1,21,1,21,
        1,21,1,21,1,21,1,21,1,22,1,22,1,22,1,22,1,22,1,22,1,23,1,23,1,23,
        1,23,1,23,1,23,1,24,1,24,1,24,1,24,1,24,3,24,397,8,24,1,25,1,25,
        1,25,3,25,402,8,25,1,26,1,26,1,26,1,26,1,26,1,26,1,26,3,26,411,8,
        26,3,26,413,8,26,1,27,1,27,1,27,1,27,1,27,1,27,1,27,3,27,422,8,27,
        1,28,1,28,3,28,426,8,28,1,29,1,29,1,29,1,29,1,29,1,29,1,30,1,30,
        1,30,1,30,1,30,1,30,1,31,1,31,1,31,3,31,443,8,31,1,32,1,32,1,32,
        1,32,5,32,449,8,32,10,32,12,32,452,9,32,1,32,1,32,5,32,456,8,32,
        10,32,12,32,459,9,32,1,32,1,32,1,32,1,32,1,32,5,32,466,8,32,10,32,
        12,32,469,9,32,1,32,1,32,1,32,1,32,5,32,475,8,32,10,32,12,32,478,
        9,32,1,32,1,32,5,32,482,8,32,10,32,12,32,485,9,32,1,32,1,32,3,32,
        489,8,32,1,32,5,32,492,8,32,10,32,12,32,495,9,32,1,32,1,32,1,32,
        1,33,1,33,1,33,1,33,1,33,1,33,1,33,1,33,3,33,508,8,33,1,34,1,34,
        3,34,512,8,34,1,34,1,34,1,35,1,35,1,35,5,35,519,8,35,10,35,12,35,
        522,9,35,1,36,1,36,1,36,3,36,527,8,36,1,36,1,36,1,36,5,36,532,8,
        36,10,36,12,36,535,9,36,1,36,5,36,538,8,36,10,36,12,36,541,9,36,
        1,37,1,37,1,37,5,37,546,8,37,10,37,12,37,549,9,37,1,37,1,37,1,37,
        5,37,554,8,37,10,37,12,37,557,9,37,1,37,1,37,5,37,561,8,37,10,37,
        12,37,564,9,37,4,37,566,8,37,11,37,12,37,567,1,37,1,37,5,37,572,
        8,37,10,37,12,37,575,9,37,1,37,1,37,5,37,579,8,37,10,37,12,37,582,
        9,37,4,37,584,8,37,11,37,12,37,585,1,37,1,37,1,38,1,38,1,38,1,38,
        1,38,1,39,1,39,1,39,1,39,3,39,599,8,39,1,40,3,40,602,8,40,1,40,1,
        40,1,40,1,40,1,40,1,40,3,40,610,8,40,1,40,1,40,1,40,1,40,3,40,616,
        8,40,3,40,618,8,40,1,41,1,41,1,41,1,41,1,41,1,41,3,41,626,8,41,1,
        42,1,42,1,42,3,42,631,8,42,1,43,1,43,1,43,1,43,1,43,1,43,1,43,1,
        43,1,43,1,43,1,43,1,43,1,43,1,43,1,43,1,43,1,43,1,43,1,43,1,43,1,
        43,1,43,1,43,1,43,1,43,1,43,1,43,1,43,1,43,1,43,1,43,1,43,1,43,1,
        43,1,43,1,43,1,43,1,43,1,43,1,43,1,43,1,43,1,43,1,43,1,43,3,43,678,
        8,43,1,44,1,44,1,45,1,45,1,45,1,45,1,45,1,45,1,45,1,45,1,45,1,45,
        1,45,1,45,1,45,1,45,1,45,1,45,1,45,1,45,1,45,1,45,1,45,1,45,1,45,
        1,45,1,45,3,45,707,8,45,1,45,1,45,1,45,1,45,5,45,713,8,45,10,45,
        12,45,716,9,45,1,46,1,46,1,47,1,47,1,48,1,48,1,48,1,48,1,48,1,48,
        1,48,1,48,1,48,1,48,1,48,0,1,90,49,0,2,4,6,8,10,12,14,16,18,20,22,
        24,26,28,30,32,34,36,38,40,42,44,46,48,50,52,54,56,58,60,62,64,66,
        68,70,72,74,76,78,80,82,84,86,88,90,92,94,96,0,5,1,0,42,43,1,0,25,
        33,1,0,42,44,2,0,47,59,62,62,2,0,45,46,62,62,784,0,101,1,0,0,0,2,
        116,1,0,0,0,4,118,1,0,0,0,6,124,1,0,0,0,8,142,1,0,0,0,10,162,1,0,
        0,0,12,164,1,0,0,0,14,226,1,0,0,0,16,240,1,0,0,0,18,259,1,0,0,0,
        20,270,1,0,0,0,22,272,1,0,0,0,24,274,1,0,0,0,26,312,1,0,0,0,28,335,
        1,0,0,0,30,337,1,0,0,0,32,343,1,0,0,0,34,349,1,0,0,0,36,355,1,0,
        0,0,38,361,1,0,0,0,40,367,1,0,0,0,42,373,1,0,0,0,44,379,1,0,0,0,
        46,385,1,0,0,0,48,396,1,0,0,0,50,401,1,0,0,0,52,412,1,0,0,0,54,421,
        1,0,0,0,56,425,1,0,0,0,58,427,1,0,0,0,60,433,1,0,0,0,62,442,1,0,
        0,0,64,444,1,0,0,0,66,507,1,0,0,0,68,509,1,0,0,0,70,515,1,0,0,0,
        72,523,1,0,0,0,74,542,1,0,0,0,76,589,1,0,0,0,78,598,1,0,0,0,80,601,
        1,0,0,0,82,619,1,0,0,0,84,630,1,0,0,0,86,677,1,0,0,0,88,679,1,0,
        0,0,90,706,1,0,0,0,92,717,1,0,0,0,94,719,1,0,0,0,96,721,1,0,0,0,
        98,100,3,2,1,0,99,98,1,0,0,0,100,103,1,0,0,0,101,99,1,0,0,0,101,
        102,1,0,0,0,102,104,1,0,0,0,103,101,1,0,0,0,104,105,5,0,0,1,105,
        1,1,0,0,0,106,117,3,4,2,0,107,117,3,6,3,0,108,117,3,28,14,0,109,
        117,3,12,6,0,110,117,3,24,12,0,111,117,3,56,28,0,112,117,3,64,32,
        0,113,117,3,74,37,0,114,117,3,96,48,0,115,117,5,65,0,0,116,106,1,
        0,0,0,116,107,1,0,0,0,116,108,1,0,0,0,116,109,1,0,0,0,116,110,1,
        0,0,0,116,111,1,0,0,0,116,112,1,0,0,0,116,113,1,0,0,0,116,114,1,
        0,0,0,116,115,1,0,0,0,117,3,1,0,0,0,118,119,5,13,0,0,119,120,3,54,
        27,0,120,121,5,61,0,0,121,122,5,64,0,0,122,123,5,60,0,0,123,5,1,
        0,0,0,124,125,5,19,0,0,125,129,5,1,0,0,126,128,5,65,0,0,127,126,
        1,0,0,0,128,131,1,0,0,0,129,127,1,0,0,0,129,130,1,0,0,0,130,132,
        1,0,0,0,131,129,1,0,0,0,132,136,3,8,4,0,133,135,5,65,0,0,134,133,
        1,0,0,0,135,138,1,0,0,0,136,134,1,0,0,0,136,137,1,0,0,0,137,139,
        1,0,0,0,138,136,1,0,0,0,139,140,5,2,0,0,140,141,5,60,0,0,141,7,1,
        0,0,0,142,153,3,10,5,0,143,147,5,3,0,0,144,146,5,65,0,0,145,144,
        1,0,0,0,146,149,1,0,0,0,147,145,1,0,0,0,147,148,1,0,0,0,148,150,
        1,0,0,0,149,147,1,0,0,0,150,152,3,8,4,0,151,143,1,0,0,0,152,155,
        1,0,0,0,153,151,1,0,0,0,153,154,1,0,0,0,154,9,1,0,0,0,155,153,1,
        0,0,0,156,157,5,20,0,0,157,158,5,61,0,0,158,163,3,50,25,0,159,160,
        5,21,0,0,160,161,5,61,0,0,161,163,3,50,25,0,162,156,1,0,0,0,162,
        159,1,0,0,0,163,11,1,0,0,0,164,165,5,14,0,0,165,166,3,54,27,0,166,
        170,5,1,0,0,167,169,5,65,0,0,168,167,1,0,0,0,169,172,1,0,0,0,170,
        168,1,0,0,0,170,171,1,0,0,0,171,173,1,0,0,0,172,170,1,0,0,0,173,
        174,5,15,0,0,174,175,5,61,0,0,175,179,5,4,0,0,176,178,5,65,0,0,177,
        176,1,0,0,0,178,181,1,0,0,0,179,177,1,0,0,0,179,180,1,0,0,0,180,
        182,1,0,0,0,181,179,1,0,0,0,182,186,3,14,7,0,183,185,5,65,0,0,184,
        183,1,0,0,0,185,188,1,0,0,0,186,184,1,0,0,0,186,187,1,0,0,0,187,
        189,1,0,0,0,188,186,1,0,0,0,189,190,5,5,0,0,190,194,5,3,0,0,191,
        193,5,65,0,0,192,191,1,0,0,0,193,196,1,0,0,0,194,192,1,0,0,0,194,
        195,1,0,0,0,195,197,1,0,0,0,196,194,1,0,0,0,197,198,5,17,0,0,198,
        199,5,61,0,0,199,203,5,4,0,0,200,202,5,65,0,0,201,200,1,0,0,0,202,
        205,1,0,0,0,203,201,1,0,0,0,203,204,1,0,0,0,204,206,1,0,0,0,205,
        203,1,0,0,0,206,210,3,16,8,0,207,209,5,65,0,0,208,207,1,0,0,0,209,
        212,1,0,0,0,210,208,1,0,0,0,210,211,1,0,0,0,211,213,1,0,0,0,212,
        210,1,0,0,0,213,215,5,5,0,0,214,216,3,18,9,0,215,214,1,0,0,0,215,
        216,1,0,0,0,216,220,1,0,0,0,217,219,5,65,0,0,218,217,1,0,0,0,219,
        222,1,0,0,0,220,218,1,0,0,0,220,221,1,0,0,0,221,223,1,0,0,0,222,
        220,1,0,0,0,223,224,5,2,0,0,224,225,5,60,0,0,225,13,1,0,0,0,226,
        237,3,54,27,0,227,231,5,3,0,0,228,230,5,65,0,0,229,228,1,0,0,0,230,
        233,1,0,0,0,231,229,1,0,0,0,231,232,1,0,0,0,232,234,1,0,0,0,233,
        231,1,0,0,0,234,236,3,14,7,0,235,227,1,0,0,0,236,239,1,0,0,0,237,
        235,1,0,0,0,237,238,1,0,0,0,238,15,1,0,0,0,239,237,1,0,0,0,240,241,
        3,54,27,0,241,243,5,6,0,0,242,244,5,18,0,0,243,242,1,0,0,0,243,244,
        1,0,0,0,244,245,1,0,0,0,245,256,3,22,11,0,246,250,5,3,0,0,247,249,
        5,65,0,0,248,247,1,0,0,0,249,252,1,0,0,0,250,248,1,0,0,0,250,251,
        1,0,0,0,251,253,1,0,0,0,252,250,1,0,0,0,253,255,3,16,8,0,254,246,
        1,0,0,0,255,258,1,0,0,0,256,254,1,0,0,0,256,257,1,0,0,0,257,17,1,
        0,0,0,258,256,1,0,0,0,259,263,5,3,0,0,260,262,5,65,0,0,261,260,1,
        0,0,0,262,265,1,0,0,0,263,261,1,0,0,0,263,264,1,0,0,0,264,266,1,
        0,0,0,265,263,1,0,0,0,266,267,5,16,0,0,267,268,5,61,0,0,268,269,
        3,20,10,0,269,19,1,0,0,0,270,271,7,0,0,0,271,21,1,0,0,0,272,273,
        7,1,0,0,273,23,1,0,0,0,274,275,5,22,0,0,275,276,5,62,0,0,276,280,
        5,1,0,0,277,279,5,65,0,0,278,277,1,0,0,0,279,282,1,0,0,0,280,278,
        1,0,0,0,280,281,1,0,0,0,281,283,1,0,0,0,282,280,1,0,0,0,283,284,
        5,23,0,0,284,285,5,61,0,0,285,289,5,4,0,0,286,288,5,65,0,0,287,286,
        1,0,0,0,288,291,1,0,0,0,289,287,1,0,0,0,289,290,1,0,0,0,290,292,
        1,0,0,0,291,289,1,0,0,0,292,296,3,26,13,0,293,295,5,65,0,0,294,293,
        1,0,0,0,295,298,1,0,0,0,296,294,1,0,0,0,296,297,1,0,0,0,297,299,
        1,0,0,0,298,296,1,0,0,0,299,301,5,5,0,0,300,302,5,3,0,0,301,300,
        1,0,0,0,301,302,1,0,0,0,302,306,1,0,0,0,303,305,5,65,0,0,304,303,
        1,0,0,0,305,308,1,0,0,0,306,304,1,0,0,0,306,307,1,0,0,0,307,309,
        1,0,0,0,308,306,1,0,0,0,309,310,5,2,0,0,310,311,5,60,0,0,311,25,
        1,0,0,0,312,323,5,64,0,0,313,317,5,3,0,0,314,316,5,65,0,0,315,314,
        1,0,0,0,316,319,1,0,0,0,317,315,1,0,0,0,317,318,1,0,0,0,318,320,
        1,0,0,0,319,317,1,0,0,0,320,322,3,26,13,0,321,313,1,0,0,0,322,325,
        1,0,0,0,323,321,1,0,0,0,323,324,1,0,0,0,324,27,1,0,0,0,325,323,1,
        0,0,0,326,336,3,30,15,0,327,336,3,32,16,0,328,336,3,34,17,0,329,
        336,3,36,18,0,330,336,3,38,19,0,331,336,3,40,20,0,332,336,3,42,21,
        0,333,336,3,44,22,0,334,336,3,46,23,0,335,326,1,0,0,0,335,327,1,
        0,0,0,335,328,1,0,0,0,335,329,1,0,0,0,335,330,1,0,0,0,335,331,1,
        0,0,0,335,332,1,0,0,0,335,333,1,0,0,0,335,334,1,0,0,0,336,29,1,0,
        0,0,337,338,5,25,0,0,338,339,3,54,27,0,339,340,5,61,0,0,340,341,
        3,48,24,0,341,342,5,60,0,0,342,31,1,0,0,0,343,344,5,26,0,0,344,345,
        3,54,27,0,345,346,5,61,0,0,346,347,3,50,25,0,347,348,5,60,0,0,348,
        33,1,0,0,0,349,350,5,27,0,0,350,351,3,54,27,0,351,352,5,61,0,0,352,
        353,3,48,24,0,353,354,5,60,0,0,354,35,1,0,0,0,355,356,5,28,0,0,356,
        357,3,54,27,0,357,358,5,61,0,0,358,359,3,50,25,0,359,360,5,60,0,
        0,360,37,1,0,0,0,361,362,5,29,0,0,362,363,3,54,27,0,363,364,5,61,
        0,0,364,365,3,52,26,0,365,366,5,60,0,0,366,39,1,0,0,0,367,368,5,
        30,0,0,368,369,3,54,27,0,369,370,5,61,0,0,370,371,5,64,0,0,371,372,
        5,60,0,0,372,41,1,0,0,0,373,374,5,31,0,0,374,375,3,54,27,0,375,376,
        5,61,0,0,376,377,5,64,0,0,377,378,5,60,0,0,378,43,1,0,0,0,379,380,
        5,32,0,0,380,381,3,54,27,0,381,382,5,61,0,0,382,383,5,64,0,0,383,
        384,5,60,0,0,384,45,1,0,0,0,385,386,5,33,0,0,386,387,3,54,27,0,387,
        388,5,61,0,0,388,389,5,64,0,0,389,390,5,60,0,0,390,47,1,0,0,0,391,
        392,5,54,0,0,392,397,3,48,24,0,393,394,5,55,0,0,394,397,3,48,24,
        0,395,397,5,63,0,0,396,391,1,0,0,0,396,393,1,0,0,0,396,395,1,0,0,
        0,397,49,1,0,0,0,398,399,5,54,0,0,399,402,3,50,25,0,400,402,5,63,
        0,0,401,398,1,0,0,0,401,400,1,0,0,0,402,51,1,0,0,0,403,404,5,54,
        0,0,404,413,3,52,26,0,405,406,5,55,0,0,406,413,3,52,26,0,407,410,
        5,63,0,0,408,409,5,7,0,0,409,411,5,63,0,0,410,408,1,0,0,0,410,411,
        1,0,0,0,411,413,1,0,0,0,412,403,1,0,0,0,412,405,1,0,0,0,412,407,
        1,0,0,0,413,53,1,0,0,0,414,415,5,62,0,0,415,416,5,8,0,0,416,422,
        5,62,0,0,417,418,5,62,0,0,418,419,5,8,0,0,419,422,5,64,0,0,420,422,
        5,62,0,0,421,414,1,0,0,0,421,417,1,0,0,0,421,420,1,0,0,0,422,55,
        1,0,0,0,423,426,3,58,29,0,424,426,3,60,30,0,425,423,1,0,0,0,425,
        424,1,0,0,0,426,57,1,0,0,0,427,428,5,34,0,0,428,429,3,54,27,0,429,
        430,5,61,0,0,430,431,3,62,31,0,431,432,5,60,0,0,432,59,1,0,0,0,433,
        434,5,35,0,0,434,435,3,54,27,0,435,436,5,61,0,0,436,437,5,64,0,0,
        437,438,5,60,0,0,438,61,1,0,0,0,439,443,3,88,44,0,440,443,5,36,0,
        0,441,443,5,64,0,0,442,439,1,0,0,0,442,440,1,0,0,0,442,441,1,0,0,
        0,443,63,1,0,0,0,444,445,5,37,0,0,445,446,3,54,27,0,446,450,5,1,
        0,0,447,449,5,65,0,0,448,447,1,0,0,0,449,452,1,0,0,0,450,448,1,0,
        0,0,450,451,1,0,0,0,451,453,1,0,0,0,452,450,1,0,0,0,453,457,3,66,
        33,0,454,456,5,65,0,0,455,454,1,0,0,0,456,459,1,0,0,0,457,455,1,
        0,0,0,457,458,1,0,0,0,458,460,1,0,0,0,459,457,1,0,0,0,460,461,5,
        40,0,0,461,462,5,61,0,0,462,463,3,68,34,0,463,467,5,3,0,0,464,466,
        5,65,0,0,465,464,1,0,0,0,466,469,1,0,0,0,467,465,1,0,0,0,467,468,
        1,0,0,0,468,470,1,0,0,0,469,467,1,0,0,0,470,471,5,41,0,0,471,472,
        5,61,0,0,472,476,5,4,0,0,473,475,5,65,0,0,474,473,1,0,0,0,475,478,
        1,0,0,0,476,474,1,0,0,0,476,477,1,0,0,0,477,479,1,0,0,0,478,476,
        1,0,0,0,479,483,3,72,36,0,480,482,5,65,0,0,481,480,1,0,0,0,482,485,
        1,0,0,0,483,481,1,0,0,0,483,484,1,0,0,0,484,486,1,0,0,0,485,483,
        1,0,0,0,486,488,5,5,0,0,487,489,5,3,0,0,488,487,1,0,0,0,488,489,
        1,0,0,0,489,493,1,0,0,0,490,492,5,65,0,0,491,490,1,0,0,0,492,495,
        1,0,0,0,493,491,1,0,0,0,493,494,1,0,0,0,494,496,1,0,0,0,495,493,
        1,0,0,0,496,497,5,2,0,0,497,498,5,60,0,0,498,65,1,0,0,0,499,500,
        5,38,0,0,500,501,5,61,0,0,501,502,5,62,0,0,502,508,5,3,0,0,503,504,
        5,39,0,0,504,505,5,61,0,0,505,506,5,64,0,0,506,508,5,3,0,0,507,499,
        1,0,0,0,507,503,1,0,0,0,508,67,1,0,0,0,509,511,5,4,0,0,510,512,3,
        70,35,0,511,510,1,0,0,0,511,512,1,0,0,0,512,513,1,0,0,0,513,514,
        5,5,0,0,514,69,1,0,0,0,515,520,5,64,0,0,516,517,5,3,0,0,517,519,
        5,64,0,0,518,516,1,0,0,0,519,522,1,0,0,0,520,518,1,0,0,0,520,521,
        1,0,0,0,521,71,1,0,0,0,522,520,1,0,0,0,523,524,5,64,0,0,524,526,
        5,6,0,0,525,527,5,18,0,0,526,525,1,0,0,0,526,527,1,0,0,0,527,528,
        1,0,0,0,528,539,3,22,11,0,529,533,5,3,0,0,530,532,5,65,0,0,531,530,
        1,0,0,0,532,535,1,0,0,0,533,531,1,0,0,0,533,534,1,0,0,0,534,536,
        1,0,0,0,535,533,1,0,0,0,536,538,3,72,36,0,537,529,1,0,0,0,538,541,
        1,0,0,0,539,537,1,0,0,0,539,540,1,0,0,0,540,73,1,0,0,0,541,539,1,
        0,0,0,542,543,5,4,0,0,543,547,5,62,0,0,544,546,3,76,38,0,545,544,
        1,0,0,0,546,549,1,0,0,0,547,545,1,0,0,0,547,548,1,0,0,0,548,550,
        1,0,0,0,549,547,1,0,0,0,550,551,5,5,0,0,551,555,5,8,0,0,552,554,
        5,65,0,0,553,552,1,0,0,0,554,557,1,0,0,0,555,553,1,0,0,0,555,556,
        1,0,0,0,556,565,1,0,0,0,557,555,1,0,0,0,558,562,3,80,40,0,559,561,
        5,65,0,0,560,559,1,0,0,0,561,564,1,0,0,0,562,560,1,0,0,0,562,563,
        1,0,0,0,563,566,1,0,0,0,564,562,1,0,0,0,565,558,1,0,0,0,566,567,
        1,0,0,0,567,565,1,0,0,0,567,568,1,0,0,0,568,569,1,0,0,0,569,573,
        5,9,0,0,570,572,5,65,0,0,571,570,1,0,0,0,572,575,1,0,0,0,573,571,
        1,0,0,0,573,574,1,0,0,0,574,583,1,0,0,0,575,573,1,0,0,0,576,580,
        3,82,41,0,577,579,5,65,0,0,578,577,1,0,0,0,579,582,1,0,0,0,580,578,
        1,0,0,0,580,581,1,0,0,0,581,584,1,0,0,0,582,580,1,0,0,0,583,576,
        1,0,0,0,584,585,1,0,0,0,585,583,1,0,0,0,585,586,1,0,0,0,586,587,
        1,0,0,0,587,588,5,60,0,0,588,75,1,0,0,0,589,590,5,3,0,0,590,591,
        5,62,0,0,591,592,5,61,0,0,592,593,3,78,39,0,593,77,1,0,0,0,594,599,
        5,64,0,0,595,599,5,42,0,0,596,599,5,43,0,0,597,599,3,48,24,0,598,
        594,1,0,0,0,598,595,1,0,0,0,598,596,1,0,0,0,598,597,1,0,0,0,599,
        79,1,0,0,0,600,602,5,45,0,0,601,600,1,0,0,0,601,602,1,0,0,0,602,
        603,1,0,0,0,603,604,5,10,0,0,604,605,3,84,42,0,605,606,3,84,42,0,
        606,607,3,86,43,0,607,609,5,11,0,0,608,610,5,7,0,0,609,608,1,0,0,
        0,609,610,1,0,0,0,610,617,1,0,0,0,611,612,5,4,0,0,612,613,3,90,45,
        0,613,615,5,5,0,0,614,616,5,7,0,0,615,614,1,0,0,0,615,616,1,0,0,
        0,616,618,1,0,0,0,617,611,1,0,0,0,617,618,1,0,0,0,618,81,1,0,0,0,
        619,620,5,10,0,0,620,621,3,84,42,0,621,622,3,84,42,0,622,623,3,90,
        45,0,623,625,5,11,0,0,624,626,5,7,0,0,625,624,1,0,0,0,625,626,1,
        0,0,0,626,83,1,0,0,0,627,628,5,12,0,0,628,631,5,62,0,0,629,631,3,
        54,27,0,630,627,1,0,0,0,630,629,1,0,0,0,631,85,1,0,0,0,632,678,3,
        84,42,0,633,634,5,25,0,0,634,635,5,10,0,0,635,636,3,48,24,0,636,
        637,5,11,0,0,637,678,1,0,0,0,638,639,5,26,0,0,639,640,5,10,0,0,640,
        641,3,50,25,0,641,642,5,11,0,0,642,678,1,0,0,0,643,644,5,27,0,0,
        644,645,5,10,0,0,645,646,3,48,24,0,646,647,5,11,0,0,647,678,1,0,
        0,0,648,649,5,28,0,0,649,650,5,10,0,0,650,651,3,50,25,0,651,652,
        5,11,0,0,652,678,1,0,0,0,653,654,5,29,0,0,654,655,5,10,0,0,655,656,
        3,52,26,0,656,657,5,11,0,0,657,678,1,0,0,0,658,659,5,30,0,0,659,
        660,5,10,0,0,660,661,5,64,0,0,661,678,5,11,0,0,662,663,5,31,0,0,
        663,664,5,10,0,0,664,665,5,64,0,0,665,678,5,11,0,0,666,667,5,32,
        0,0,667,668,5,10,0,0,668,669,5,64,0,0,669,678,5,11,0,0,670,671,5,
        33,0,0,671,672,5,10,0,0,672,673,5,64,0,0,673,678,5,11,0,0,674,678,
        5,64,0,0,675,678,3,88,44,0,676,678,3,52,26,0,677,632,1,0,0,0,677,
        633,1,0,0,0,677,638,1,0,0,0,677,643,1,0,0,0,677,648,1,0,0,0,677,
        653,1,0,0,0,677,658,1,0,0,0,677,662,1,0,0,0,677,666,1,0,0,0,677,
        670,1,0,0,0,677,674,1,0,0,0,677,675,1,0,0,0,677,676,1,0,0,0,678,
        87,1,0,0,0,679,680,7,2,0,0,680,89,1,0,0,0,681,682,6,45,-1,0,682,
        683,5,10,0,0,683,684,3,90,45,0,684,685,3,92,46,0,685,686,3,90,45,
        0,686,687,5,11,0,0,687,707,1,0,0,0,688,689,3,94,47,0,689,690,5,10,
        0,0,690,691,3,90,45,0,691,692,5,11,0,0,692,707,1,0,0,0,693,694,5,
        10,0,0,694,695,3,94,47,0,695,696,3,90,45,0,696,697,5,11,0,0,697,
        707,1,0,0,0,698,699,5,10,0,0,699,700,3,90,45,0,700,701,5,11,0,0,
        701,707,1,0,0,0,702,703,3,94,47,0,703,704,3,90,45,2,704,707,1,0,
        0,0,705,707,3,86,43,0,706,681,1,0,0,0,706,688,1,0,0,0,706,693,1,
        0,0,0,706,698,1,0,0,0,706,702,1,0,0,0,706,705,1,0,0,0,707,714,1,
        0,0,0,708,709,10,7,0,0,709,710,3,92,46,0,710,711,3,90,45,8,711,713,
        1,0,0,0,712,708,1,0,0,0,713,716,1,0,0,0,714,712,1,0,0,0,714,715,
        1,0,0,0,715,91,1,0,0,0,716,714,1,0,0,0,717,718,7,3,0,0,718,93,1,
        0,0,0,719,720,7,4,0,0,720,95,1,0,0,0,721,722,5,24,0,0,722,723,5,
        10,0,0,723,724,3,84,42,0,724,725,5,3,0,0,725,726,3,84,42,0,726,727,
        5,3,0,0,727,728,3,86,43,0,728,729,5,11,0,0,729,730,5,60,0,0,730,
        97,1,0,0,0,66,101,116,129,136,147,153,162,170,179,186,194,203,210,
        215,220,231,237,243,250,256,263,280,289,296,301,306,317,323,335,
        396,401,410,412,421,425,442,450,457,467,476,483,488,493,507,511,
        520,526,533,539,547,555,562,567,573,580,585,598,601,609,615,617,
        625,630,677,706,714
    ]

class JetRuleParser ( Parser ):

    grammarFileName = "JetRule.g4"

    atn = ATNDeserializer().deserialize(serializedATN())

    decisionsToDFA = [ DFA(ds, i) for i, ds in enumerate(atn.decisionToState) ]

    sharedContextCache = PredictionContextCache()

    literalNames = [ "<INVALID>", "'{'", "'}'", "','", "'['", "']'", "'as'", 
                     "'.'", "':'", "'->'", "'('", "')'", "'?'", "'@JetCompilerDirective'", 
                     "'class'", "'$base_classes'", "'$as_table'", "'$data_properties'", 
                     "'array of'", "'jetstore_config'", "'$max_looping'", 
                     "'$max_rule_exec'", "'rule_sequence'", "'$main_rule_sets'", 
                     "'triple'", "'int'", "'uint'", "'long'", "'ulong'", 
                     "'double'", "'text'", "'date'", "'datetime'", "'bool'", 
                     "'resource'", "'volatile_resource'", "'create_uuid_resource()'", 
                     "'lookup_table'", "'$table_name'", "'$csv_file'", "'$key'", 
                     "'$columns'", "'true'", "'false'", "'null'", "'not'", 
                     "'toText'", "'=='", "'<'", "'<='", "'>'", "'>='", "'!='", 
                     "'r?'", "'+'", "'-'", "'*'", "'/'", "'or'", "'and'", 
                     "';'", "'='" ]

    symbolicNames = [ "<INVALID>", "<INVALID>", "<INVALID>", "<INVALID>", 
                      "<INVALID>", "<INVALID>", "<INVALID>", "<INVALID>", 
                      "<INVALID>", "<INVALID>", "<INVALID>", "<INVALID>", 
                      "<INVALID>", "JetCompilerDirective", "CLASS", "BaseClasses", 
                      "AsTable", "DataProperties", "ARRAY", "JETSCONFIG", 
                      "MaxLooping", "MaxRuleExec", "RULESEQ", "MainRuleSets", 
                      "TRIPLE", "Int32Type", "UInt32Type", "Int64Type", 
                      "UInt64Type", "DoubleType", "StringType", "DateType", 
                      "DatetimeType", "BoolType", "ResourceType", "VolatileResourceType", 
                      "CreateUUIDResource", "LookupTable", "TableName", 
                      "CSVFileName", "Key", "Columns", "TRUE", "FALSE", 
                      "NULL", "NOT", "TOTEXT", "EQ", "LT", "LE", "GT", "GE", 
                      "NE", "REGEX2", "PLUS", "MINUS", "MUL", "DIV", "OR", 
                      "AND", "SEMICOLON", "ASSIGN", "Identifier", "DIGITS", 
                      "STRING", "COMMENT", "WS" ]

    RULE_jetrule = 0
    RULE_statement = 1
    RULE_jetCompilerDirectiveStmt = 2
    RULE_defineJetStoreConfigStmt = 3
    RULE_jetstoreConfigSeq = 4
    RULE_jetstoreConfigItem = 5
    RULE_defineClassStmt = 6
    RULE_subClassOfStmt = 7
    RULE_dataPropertyDefinitions = 8
    RULE_asTableStmt = 9
    RULE_asTableFlag = 10
    RULE_dataPropertyType = 11
    RULE_defineRuleSeqStmt = 12
    RULE_ruleSetDefinitions = 13
    RULE_defineLiteralStmt = 14
    RULE_int32LiteralStmt = 15
    RULE_uInt32LiteralStmt = 16
    RULE_int64LiteralStmt = 17
    RULE_uInt64LiteralStmt = 18
    RULE_doubleLiteralStmt = 19
    RULE_stringLiteralStmt = 20
    RULE_dateLiteralStmt = 21
    RULE_datetimeLiteralStmt = 22
    RULE_booleanLiteralStmt = 23
    RULE_intExpr = 24
    RULE_uintExpr = 25
    RULE_doubleExpr = 26
    RULE_declIdentifier = 27
    RULE_defineResourceStmt = 28
    RULE_namedResourceStmt = 29
    RULE_volatileResourceStmt = 30
    RULE_resourceValue = 31
    RULE_lookupTableStmt = 32
    RULE_csvLocation = 33
    RULE_stringList = 34
    RULE_stringSeq = 35
    RULE_columnDefinitions = 36
    RULE_jetRuleStmt = 37
    RULE_ruleProperties = 38
    RULE_propertyValue = 39
    RULE_antecedent = 40
    RULE_consequent = 41
    RULE_atom = 42
    RULE_objectAtom = 43
    RULE_keywords = 44
    RULE_exprTerm = 45
    RULE_binaryOp = 46
    RULE_unaryOp = 47
    RULE_tripleStmt = 48

    ruleNames =  [ "jetrule", "statement", "jetCompilerDirectiveStmt", "defineJetStoreConfigStmt", 
                   "jetstoreConfigSeq", "jetstoreConfigItem", "defineClassStmt", 
                   "subClassOfStmt", "dataPropertyDefinitions", "asTableStmt", 
                   "asTableFlag", "dataPropertyType", "defineRuleSeqStmt", 
                   "ruleSetDefinitions", "defineLiteralStmt", "int32LiteralStmt", 
                   "uInt32LiteralStmt", "int64LiteralStmt", "uInt64LiteralStmt", 
                   "doubleLiteralStmt", "stringLiteralStmt", "dateLiteralStmt", 
                   "datetimeLiteralStmt", "booleanLiteralStmt", "intExpr", 
                   "uintExpr", "doubleExpr", "declIdentifier", "defineResourceStmt", 
                   "namedResourceStmt", "volatileResourceStmt", "resourceValue", 
                   "lookupTableStmt", "csvLocation", "stringList", "stringSeq", 
                   "columnDefinitions", "jetRuleStmt", "ruleProperties", 
                   "propertyValue", "antecedent", "consequent", "atom", 
                   "objectAtom", "keywords", "exprTerm", "binaryOp", "unaryOp", 
                   "tripleStmt" ]

    EOF = Token.EOF
    T__0=1
    T__1=2
    T__2=3
    T__3=4
    T__4=5
    T__5=6
    T__6=7
    T__7=8
    T__8=9
    T__9=10
    T__10=11
    T__11=12
    JetCompilerDirective=13
    CLASS=14
    BaseClasses=15
    AsTable=16
    DataProperties=17
    ARRAY=18
    JETSCONFIG=19
    MaxLooping=20
    MaxRuleExec=21
    RULESEQ=22
    MainRuleSets=23
    TRIPLE=24
    Int32Type=25
    UInt32Type=26
    Int64Type=27
    UInt64Type=28
    DoubleType=29
    StringType=30
    DateType=31
    DatetimeType=32
    BoolType=33
    ResourceType=34
    VolatileResourceType=35
    CreateUUIDResource=36
    LookupTable=37
    TableName=38
    CSVFileName=39
    Key=40
    Columns=41
    TRUE=42
    FALSE=43
    NULL=44
    NOT=45
    TOTEXT=46
    EQ=47
    LT=48
    LE=49
    GT=50
    GE=51
    NE=52
    REGEX2=53
    PLUS=54
    MINUS=55
    MUL=56
    DIV=57
    OR=58
    AND=59
    SEMICOLON=60
    ASSIGN=61
    Identifier=62
    DIGITS=63
    STRING=64
    COMMENT=65
    WS=66

    def __init__(self, input:TokenStream, output:TextIO = sys.stdout):
        super().__init__(input, output)
        self.checkVersion("4.10.1")
        self._interp = ParserATNSimulator(self, self.atn, self.decisionsToDFA, self.sharedContextCache)
        self._predicates = None




    class JetruleContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser

        def EOF(self):
            return self.getToken(JetRuleParser.EOF, 0)

        def statement(self, i:int=None):
            if i is None:
                return self.getTypedRuleContexts(JetRuleParser.StatementContext)
            else:
                return self.getTypedRuleContext(JetRuleParser.StatementContext,i)


        def getRuleIndex(self):
            return JetRuleParser.RULE_jetrule

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterJetrule" ):
                listener.enterJetrule(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitJetrule" ):
                listener.exitJetrule(self)




    def jetrule(self):

        localctx = JetRuleParser.JetruleContext(self, self._ctx, self.state)
        self.enterRule(localctx, 0, self.RULE_jetrule)
        self._la = 0 # Token type
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 101
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while ((((_la - 4)) & ~0x3f) == 0 and ((1 << (_la - 4)) & ((1 << (JetRuleParser.T__3 - 4)) | (1 << (JetRuleParser.JetCompilerDirective - 4)) | (1 << (JetRuleParser.CLASS - 4)) | (1 << (JetRuleParser.JETSCONFIG - 4)) | (1 << (JetRuleParser.RULESEQ - 4)) | (1 << (JetRuleParser.TRIPLE - 4)) | (1 << (JetRuleParser.Int32Type - 4)) | (1 << (JetRuleParser.UInt32Type - 4)) | (1 << (JetRuleParser.Int64Type - 4)) | (1 << (JetRuleParser.UInt64Type - 4)) | (1 << (JetRuleParser.DoubleType - 4)) | (1 << (JetRuleParser.StringType - 4)) | (1 << (JetRuleParser.DateType - 4)) | (1 << (JetRuleParser.DatetimeType - 4)) | (1 << (JetRuleParser.BoolType - 4)) | (1 << (JetRuleParser.ResourceType - 4)) | (1 << (JetRuleParser.VolatileResourceType - 4)) | (1 << (JetRuleParser.LookupTable - 4)) | (1 << (JetRuleParser.COMMENT - 4)))) != 0):
                self.state = 98
                self.statement()
                self.state = 103
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 104
            self.match(JetRuleParser.EOF)
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class StatementContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser

        def jetCompilerDirectiveStmt(self):
            return self.getTypedRuleContext(JetRuleParser.JetCompilerDirectiveStmtContext,0)


        def defineJetStoreConfigStmt(self):
            return self.getTypedRuleContext(JetRuleParser.DefineJetStoreConfigStmtContext,0)


        def defineLiteralStmt(self):
            return self.getTypedRuleContext(JetRuleParser.DefineLiteralStmtContext,0)


        def defineClassStmt(self):
            return self.getTypedRuleContext(JetRuleParser.DefineClassStmtContext,0)


        def defineRuleSeqStmt(self):
            return self.getTypedRuleContext(JetRuleParser.DefineRuleSeqStmtContext,0)


        def defineResourceStmt(self):
            return self.getTypedRuleContext(JetRuleParser.DefineResourceStmtContext,0)


        def lookupTableStmt(self):
            return self.getTypedRuleContext(JetRuleParser.LookupTableStmtContext,0)


        def jetRuleStmt(self):
            return self.getTypedRuleContext(JetRuleParser.JetRuleStmtContext,0)


        def tripleStmt(self):
            return self.getTypedRuleContext(JetRuleParser.TripleStmtContext,0)


        def COMMENT(self):
            return self.getToken(JetRuleParser.COMMENT, 0)

        def getRuleIndex(self):
            return JetRuleParser.RULE_statement

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterStatement" ):
                listener.enterStatement(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitStatement" ):
                listener.exitStatement(self)




    def statement(self):

        localctx = JetRuleParser.StatementContext(self, self._ctx, self.state)
        self.enterRule(localctx, 2, self.RULE_statement)
        try:
            self.state = 116
            self._errHandler.sync(self)
            token = self._input.LA(1)
            if token in [JetRuleParser.JetCompilerDirective]:
                self.enterOuterAlt(localctx, 1)
                self.state = 106
                self.jetCompilerDirectiveStmt()
                pass
            elif token in [JetRuleParser.JETSCONFIG]:
                self.enterOuterAlt(localctx, 2)
                self.state = 107
                self.defineJetStoreConfigStmt()
                pass
            elif token in [JetRuleParser.Int32Type, JetRuleParser.UInt32Type, JetRuleParser.Int64Type, JetRuleParser.UInt64Type, JetRuleParser.DoubleType, JetRuleParser.StringType, JetRuleParser.DateType, JetRuleParser.DatetimeType, JetRuleParser.BoolType]:
                self.enterOuterAlt(localctx, 3)
                self.state = 108
                self.defineLiteralStmt()
                pass
            elif token in [JetRuleParser.CLASS]:
                self.enterOuterAlt(localctx, 4)
                self.state = 109
                self.defineClassStmt()
                pass
            elif token in [JetRuleParser.RULESEQ]:
                self.enterOuterAlt(localctx, 5)
                self.state = 110
                self.defineRuleSeqStmt()
                pass
            elif token in [JetRuleParser.ResourceType, JetRuleParser.VolatileResourceType]:
                self.enterOuterAlt(localctx, 6)
                self.state = 111
                self.defineResourceStmt()
                pass
            elif token in [JetRuleParser.LookupTable]:
                self.enterOuterAlt(localctx, 7)
                self.state = 112
                self.lookupTableStmt()
                pass
            elif token in [JetRuleParser.T__3]:
                self.enterOuterAlt(localctx, 8)
                self.state = 113
                self.jetRuleStmt()
                pass
            elif token in [JetRuleParser.TRIPLE]:
                self.enterOuterAlt(localctx, 9)
                self.state = 114
                self.tripleStmt()
                pass
            elif token in [JetRuleParser.COMMENT]:
                self.enterOuterAlt(localctx, 10)
                self.state = 115
                self.match(JetRuleParser.COMMENT)
                pass
            else:
                raise NoViableAltException(self)

        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class JetCompilerDirectiveStmtContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.varName = None # DeclIdentifierContext
            self.declValue = None # Token

        def JetCompilerDirective(self):
            return self.getToken(JetRuleParser.JetCompilerDirective, 0)

        def ASSIGN(self):
            return self.getToken(JetRuleParser.ASSIGN, 0)

        def SEMICOLON(self):
            return self.getToken(JetRuleParser.SEMICOLON, 0)

        def declIdentifier(self):
            return self.getTypedRuleContext(JetRuleParser.DeclIdentifierContext,0)


        def STRING(self):
            return self.getToken(JetRuleParser.STRING, 0)

        def getRuleIndex(self):
            return JetRuleParser.RULE_jetCompilerDirectiveStmt

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterJetCompilerDirectiveStmt" ):
                listener.enterJetCompilerDirectiveStmt(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitJetCompilerDirectiveStmt" ):
                listener.exitJetCompilerDirectiveStmt(self)




    def jetCompilerDirectiveStmt(self):

        localctx = JetRuleParser.JetCompilerDirectiveStmtContext(self, self._ctx, self.state)
        self.enterRule(localctx, 4, self.RULE_jetCompilerDirectiveStmt)
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 118
            self.match(JetRuleParser.JetCompilerDirective)
            self.state = 119
            localctx.varName = self.declIdentifier()
            self.state = 120
            self.match(JetRuleParser.ASSIGN)
            self.state = 121
            localctx.declValue = self.match(JetRuleParser.STRING)
            self.state = 122
            self.match(JetRuleParser.SEMICOLON)
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class DefineJetStoreConfigStmtContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser

        def JETSCONFIG(self):
            return self.getToken(JetRuleParser.JETSCONFIG, 0)

        def jetstoreConfigSeq(self):
            return self.getTypedRuleContext(JetRuleParser.JetstoreConfigSeqContext,0)


        def SEMICOLON(self):
            return self.getToken(JetRuleParser.SEMICOLON, 0)

        def COMMENT(self, i:int=None):
            if i is None:
                return self.getTokens(JetRuleParser.COMMENT)
            else:
                return self.getToken(JetRuleParser.COMMENT, i)

        def getRuleIndex(self):
            return JetRuleParser.RULE_defineJetStoreConfigStmt

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterDefineJetStoreConfigStmt" ):
                listener.enterDefineJetStoreConfigStmt(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitDefineJetStoreConfigStmt" ):
                listener.exitDefineJetStoreConfigStmt(self)




    def defineJetStoreConfigStmt(self):

        localctx = JetRuleParser.DefineJetStoreConfigStmtContext(self, self._ctx, self.state)
        self.enterRule(localctx, 6, self.RULE_defineJetStoreConfigStmt)
        self._la = 0 # Token type
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 124
            self.match(JetRuleParser.JETSCONFIG)
            self.state = 125
            self.match(JetRuleParser.T__0)
            self.state = 129
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.COMMENT:
                self.state = 126
                self.match(JetRuleParser.COMMENT)
                self.state = 131
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 132
            self.jetstoreConfigSeq()
            self.state = 136
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.COMMENT:
                self.state = 133
                self.match(JetRuleParser.COMMENT)
                self.state = 138
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 139
            self.match(JetRuleParser.T__1)
            self.state = 140
            self.match(JetRuleParser.SEMICOLON)
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class JetstoreConfigSeqContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser

        def jetstoreConfigItem(self):
            return self.getTypedRuleContext(JetRuleParser.JetstoreConfigItemContext,0)


        def jetstoreConfigSeq(self, i:int=None):
            if i is None:
                return self.getTypedRuleContexts(JetRuleParser.JetstoreConfigSeqContext)
            else:
                return self.getTypedRuleContext(JetRuleParser.JetstoreConfigSeqContext,i)


        def COMMENT(self, i:int=None):
            if i is None:
                return self.getTokens(JetRuleParser.COMMENT)
            else:
                return self.getToken(JetRuleParser.COMMENT, i)

        def getRuleIndex(self):
            return JetRuleParser.RULE_jetstoreConfigSeq

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterJetstoreConfigSeq" ):
                listener.enterJetstoreConfigSeq(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitJetstoreConfigSeq" ):
                listener.exitJetstoreConfigSeq(self)




    def jetstoreConfigSeq(self):

        localctx = JetRuleParser.JetstoreConfigSeqContext(self, self._ctx, self.state)
        self.enterRule(localctx, 8, self.RULE_jetstoreConfigSeq)
        self._la = 0 # Token type
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 142
            self.jetstoreConfigItem()
            self.state = 153
            self._errHandler.sync(self)
            _alt = self._interp.adaptivePredict(self._input,5,self._ctx)
            while _alt!=2 and _alt!=ATN.INVALID_ALT_NUMBER:
                if _alt==1:
                    self.state = 143
                    self.match(JetRuleParser.T__2)
                    self.state = 147
                    self._errHandler.sync(self)
                    _la = self._input.LA(1)
                    while _la==JetRuleParser.COMMENT:
                        self.state = 144
                        self.match(JetRuleParser.COMMENT)
                        self.state = 149
                        self._errHandler.sync(self)
                        _la = self._input.LA(1)

                    self.state = 150
                    self.jetstoreConfigSeq() 
                self.state = 155
                self._errHandler.sync(self)
                _alt = self._interp.adaptivePredict(self._input,5,self._ctx)

        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class JetstoreConfigItemContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.configKey = None # Token
            self.configValue = None # UintExprContext

        def ASSIGN(self):
            return self.getToken(JetRuleParser.ASSIGN, 0)

        def MaxLooping(self):
            return self.getToken(JetRuleParser.MaxLooping, 0)

        def uintExpr(self):
            return self.getTypedRuleContext(JetRuleParser.UintExprContext,0)


        def MaxRuleExec(self):
            return self.getToken(JetRuleParser.MaxRuleExec, 0)

        def getRuleIndex(self):
            return JetRuleParser.RULE_jetstoreConfigItem

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterJetstoreConfigItem" ):
                listener.enterJetstoreConfigItem(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitJetstoreConfigItem" ):
                listener.exitJetstoreConfigItem(self)




    def jetstoreConfigItem(self):

        localctx = JetRuleParser.JetstoreConfigItemContext(self, self._ctx, self.state)
        self.enterRule(localctx, 10, self.RULE_jetstoreConfigItem)
        try:
            self.state = 162
            self._errHandler.sync(self)
            token = self._input.LA(1)
            if token in [JetRuleParser.MaxLooping]:
                self.enterOuterAlt(localctx, 1)
                self.state = 156
                localctx.configKey = self.match(JetRuleParser.MaxLooping)
                self.state = 157
                self.match(JetRuleParser.ASSIGN)
                self.state = 158
                localctx.configValue = self.uintExpr()
                pass
            elif token in [JetRuleParser.MaxRuleExec]:
                self.enterOuterAlt(localctx, 2)
                self.state = 159
                localctx.configKey = self.match(JetRuleParser.MaxRuleExec)
                self.state = 160
                self.match(JetRuleParser.ASSIGN)
                self.state = 161
                localctx.configValue = self.uintExpr()
                pass
            else:
                raise NoViableAltException(self)

        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class DefineClassStmtContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.className = None # DeclIdentifierContext

        def CLASS(self):
            return self.getToken(JetRuleParser.CLASS, 0)

        def BaseClasses(self):
            return self.getToken(JetRuleParser.BaseClasses, 0)

        def ASSIGN(self, i:int=None):
            if i is None:
                return self.getTokens(JetRuleParser.ASSIGN)
            else:
                return self.getToken(JetRuleParser.ASSIGN, i)

        def subClassOfStmt(self):
            return self.getTypedRuleContext(JetRuleParser.SubClassOfStmtContext,0)


        def DataProperties(self):
            return self.getToken(JetRuleParser.DataProperties, 0)

        def dataPropertyDefinitions(self):
            return self.getTypedRuleContext(JetRuleParser.DataPropertyDefinitionsContext,0)


        def SEMICOLON(self):
            return self.getToken(JetRuleParser.SEMICOLON, 0)

        def declIdentifier(self):
            return self.getTypedRuleContext(JetRuleParser.DeclIdentifierContext,0)


        def COMMENT(self, i:int=None):
            if i is None:
                return self.getTokens(JetRuleParser.COMMENT)
            else:
                return self.getToken(JetRuleParser.COMMENT, i)

        def asTableStmt(self):
            return self.getTypedRuleContext(JetRuleParser.AsTableStmtContext,0)


        def getRuleIndex(self):
            return JetRuleParser.RULE_defineClassStmt

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterDefineClassStmt" ):
                listener.enterDefineClassStmt(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitDefineClassStmt" ):
                listener.exitDefineClassStmt(self)




    def defineClassStmt(self):

        localctx = JetRuleParser.DefineClassStmtContext(self, self._ctx, self.state)
        self.enterRule(localctx, 12, self.RULE_defineClassStmt)
        self._la = 0 # Token type
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 164
            self.match(JetRuleParser.CLASS)
            self.state = 165
            localctx.className = self.declIdentifier()
            self.state = 166
            self.match(JetRuleParser.T__0)
            self.state = 170
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.COMMENT:
                self.state = 167
                self.match(JetRuleParser.COMMENT)
                self.state = 172
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 173
            self.match(JetRuleParser.BaseClasses)
            self.state = 174
            self.match(JetRuleParser.ASSIGN)
            self.state = 175
            self.match(JetRuleParser.T__3)
            self.state = 179
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.COMMENT:
                self.state = 176
                self.match(JetRuleParser.COMMENT)
                self.state = 181
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 182
            self.subClassOfStmt()
            self.state = 186
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.COMMENT:
                self.state = 183
                self.match(JetRuleParser.COMMENT)
                self.state = 188
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 189
            self.match(JetRuleParser.T__4)
            self.state = 190
            self.match(JetRuleParser.T__2)
            self.state = 194
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.COMMENT:
                self.state = 191
                self.match(JetRuleParser.COMMENT)
                self.state = 196
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 197
            self.match(JetRuleParser.DataProperties)
            self.state = 198
            self.match(JetRuleParser.ASSIGN)
            self.state = 199
            self.match(JetRuleParser.T__3)
            self.state = 203
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.COMMENT:
                self.state = 200
                self.match(JetRuleParser.COMMENT)
                self.state = 205
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 206
            self.dataPropertyDefinitions()
            self.state = 210
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.COMMENT:
                self.state = 207
                self.match(JetRuleParser.COMMENT)
                self.state = 212
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 213
            self.match(JetRuleParser.T__4)
            self.state = 215
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            if _la==JetRuleParser.T__2:
                self.state = 214
                self.asTableStmt()


            self.state = 220
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.COMMENT:
                self.state = 217
                self.match(JetRuleParser.COMMENT)
                self.state = 222
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 223
            self.match(JetRuleParser.T__1)
            self.state = 224
            self.match(JetRuleParser.SEMICOLON)
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class SubClassOfStmtContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.baseClassName = None # DeclIdentifierContext

        def declIdentifier(self):
            return self.getTypedRuleContext(JetRuleParser.DeclIdentifierContext,0)


        def subClassOfStmt(self, i:int=None):
            if i is None:
                return self.getTypedRuleContexts(JetRuleParser.SubClassOfStmtContext)
            else:
                return self.getTypedRuleContext(JetRuleParser.SubClassOfStmtContext,i)


        def COMMENT(self, i:int=None):
            if i is None:
                return self.getTokens(JetRuleParser.COMMENT)
            else:
                return self.getToken(JetRuleParser.COMMENT, i)

        def getRuleIndex(self):
            return JetRuleParser.RULE_subClassOfStmt

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterSubClassOfStmt" ):
                listener.enterSubClassOfStmt(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitSubClassOfStmt" ):
                listener.exitSubClassOfStmt(self)




    def subClassOfStmt(self):

        localctx = JetRuleParser.SubClassOfStmtContext(self, self._ctx, self.state)
        self.enterRule(localctx, 14, self.RULE_subClassOfStmt)
        self._la = 0 # Token type
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 226
            localctx.baseClassName = self.declIdentifier()
            self.state = 237
            self._errHandler.sync(self)
            _alt = self._interp.adaptivePredict(self._input,16,self._ctx)
            while _alt!=2 and _alt!=ATN.INVALID_ALT_NUMBER:
                if _alt==1:
                    self.state = 227
                    self.match(JetRuleParser.T__2)
                    self.state = 231
                    self._errHandler.sync(self)
                    _la = self._input.LA(1)
                    while _la==JetRuleParser.COMMENT:
                        self.state = 228
                        self.match(JetRuleParser.COMMENT)
                        self.state = 233
                        self._errHandler.sync(self)
                        _la = self._input.LA(1)

                    self.state = 234
                    self.subClassOfStmt() 
                self.state = 239
                self._errHandler.sync(self)
                _alt = self._interp.adaptivePredict(self._input,16,self._ctx)

        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class DataPropertyDefinitionsContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.dataPName = None # DeclIdentifierContext
            self.array = None # Token
            self.dataPType = None # DataPropertyTypeContext

        def declIdentifier(self):
            return self.getTypedRuleContext(JetRuleParser.DeclIdentifierContext,0)


        def dataPropertyType(self):
            return self.getTypedRuleContext(JetRuleParser.DataPropertyTypeContext,0)


        def dataPropertyDefinitions(self, i:int=None):
            if i is None:
                return self.getTypedRuleContexts(JetRuleParser.DataPropertyDefinitionsContext)
            else:
                return self.getTypedRuleContext(JetRuleParser.DataPropertyDefinitionsContext,i)


        def ARRAY(self):
            return self.getToken(JetRuleParser.ARRAY, 0)

        def COMMENT(self, i:int=None):
            if i is None:
                return self.getTokens(JetRuleParser.COMMENT)
            else:
                return self.getToken(JetRuleParser.COMMENT, i)

        def getRuleIndex(self):
            return JetRuleParser.RULE_dataPropertyDefinitions

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterDataPropertyDefinitions" ):
                listener.enterDataPropertyDefinitions(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitDataPropertyDefinitions" ):
                listener.exitDataPropertyDefinitions(self)




    def dataPropertyDefinitions(self):

        localctx = JetRuleParser.DataPropertyDefinitionsContext(self, self._ctx, self.state)
        self.enterRule(localctx, 16, self.RULE_dataPropertyDefinitions)
        self._la = 0 # Token type
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 240
            localctx.dataPName = self.declIdentifier()
            self.state = 241
            self.match(JetRuleParser.T__5)
            self.state = 243
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            if _la==JetRuleParser.ARRAY:
                self.state = 242
                localctx.array = self.match(JetRuleParser.ARRAY)


            self.state = 245
            localctx.dataPType = self.dataPropertyType()
            self.state = 256
            self._errHandler.sync(self)
            _alt = self._interp.adaptivePredict(self._input,19,self._ctx)
            while _alt!=2 and _alt!=ATN.INVALID_ALT_NUMBER:
                if _alt==1:
                    self.state = 246
                    self.match(JetRuleParser.T__2)
                    self.state = 250
                    self._errHandler.sync(self)
                    _la = self._input.LA(1)
                    while _la==JetRuleParser.COMMENT:
                        self.state = 247
                        self.match(JetRuleParser.COMMENT)
                        self.state = 252
                        self._errHandler.sync(self)
                        _la = self._input.LA(1)

                    self.state = 253
                    self.dataPropertyDefinitions() 
                self.state = 258
                self._errHandler.sync(self)
                _alt = self._interp.adaptivePredict(self._input,19,self._ctx)

        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class AsTableStmtContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.asTable = None # AsTableFlagContext

        def AsTable(self):
            return self.getToken(JetRuleParser.AsTable, 0)

        def ASSIGN(self):
            return self.getToken(JetRuleParser.ASSIGN, 0)

        def asTableFlag(self):
            return self.getTypedRuleContext(JetRuleParser.AsTableFlagContext,0)


        def COMMENT(self, i:int=None):
            if i is None:
                return self.getTokens(JetRuleParser.COMMENT)
            else:
                return self.getToken(JetRuleParser.COMMENT, i)

        def getRuleIndex(self):
            return JetRuleParser.RULE_asTableStmt

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterAsTableStmt" ):
                listener.enterAsTableStmt(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitAsTableStmt" ):
                listener.exitAsTableStmt(self)




    def asTableStmt(self):

        localctx = JetRuleParser.AsTableStmtContext(self, self._ctx, self.state)
        self.enterRule(localctx, 18, self.RULE_asTableStmt)
        self._la = 0 # Token type
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 259
            self.match(JetRuleParser.T__2)
            self.state = 263
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.COMMENT:
                self.state = 260
                self.match(JetRuleParser.COMMENT)
                self.state = 265
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 266
            self.match(JetRuleParser.AsTable)
            self.state = 267
            self.match(JetRuleParser.ASSIGN)
            self.state = 268
            localctx.asTable = self.asTableFlag()
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class AsTableFlagContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser

        def TRUE(self):
            return self.getToken(JetRuleParser.TRUE, 0)

        def FALSE(self):
            return self.getToken(JetRuleParser.FALSE, 0)

        def getRuleIndex(self):
            return JetRuleParser.RULE_asTableFlag

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterAsTableFlag" ):
                listener.enterAsTableFlag(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitAsTableFlag" ):
                listener.exitAsTableFlag(self)




    def asTableFlag(self):

        localctx = JetRuleParser.AsTableFlagContext(self, self._ctx, self.state)
        self.enterRule(localctx, 20, self.RULE_asTableFlag)
        self._la = 0 # Token type
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 270
            _la = self._input.LA(1)
            if not(_la==JetRuleParser.TRUE or _la==JetRuleParser.FALSE):
                self._errHandler.recoverInline(self)
            else:
                self._errHandler.reportMatch(self)
                self.consume()
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class DataPropertyTypeContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser

        def Int32Type(self):
            return self.getToken(JetRuleParser.Int32Type, 0)

        def UInt32Type(self):
            return self.getToken(JetRuleParser.UInt32Type, 0)

        def Int64Type(self):
            return self.getToken(JetRuleParser.Int64Type, 0)

        def UInt64Type(self):
            return self.getToken(JetRuleParser.UInt64Type, 0)

        def DoubleType(self):
            return self.getToken(JetRuleParser.DoubleType, 0)

        def StringType(self):
            return self.getToken(JetRuleParser.StringType, 0)

        def DateType(self):
            return self.getToken(JetRuleParser.DateType, 0)

        def DatetimeType(self):
            return self.getToken(JetRuleParser.DatetimeType, 0)

        def BoolType(self):
            return self.getToken(JetRuleParser.BoolType, 0)

        def getRuleIndex(self):
            return JetRuleParser.RULE_dataPropertyType

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterDataPropertyType" ):
                listener.enterDataPropertyType(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitDataPropertyType" ):
                listener.exitDataPropertyType(self)




    def dataPropertyType(self):

        localctx = JetRuleParser.DataPropertyTypeContext(self, self._ctx, self.state)
        self.enterRule(localctx, 22, self.RULE_dataPropertyType)
        self._la = 0 # Token type
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 272
            _la = self._input.LA(1)
            if not((((_la) & ~0x3f) == 0 and ((1 << _la) & ((1 << JetRuleParser.Int32Type) | (1 << JetRuleParser.UInt32Type) | (1 << JetRuleParser.Int64Type) | (1 << JetRuleParser.UInt64Type) | (1 << JetRuleParser.DoubleType) | (1 << JetRuleParser.StringType) | (1 << JetRuleParser.DateType) | (1 << JetRuleParser.DatetimeType) | (1 << JetRuleParser.BoolType))) != 0)):
                self._errHandler.recoverInline(self)
            else:
                self._errHandler.reportMatch(self)
                self.consume()
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class DefineRuleSeqStmtContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.ruleseqName = None # Token

        def RULESEQ(self):
            return self.getToken(JetRuleParser.RULESEQ, 0)

        def MainRuleSets(self):
            return self.getToken(JetRuleParser.MainRuleSets, 0)

        def ASSIGN(self):
            return self.getToken(JetRuleParser.ASSIGN, 0)

        def ruleSetDefinitions(self):
            return self.getTypedRuleContext(JetRuleParser.RuleSetDefinitionsContext,0)


        def SEMICOLON(self):
            return self.getToken(JetRuleParser.SEMICOLON, 0)

        def Identifier(self):
            return self.getToken(JetRuleParser.Identifier, 0)

        def COMMENT(self, i:int=None):
            if i is None:
                return self.getTokens(JetRuleParser.COMMENT)
            else:
                return self.getToken(JetRuleParser.COMMENT, i)

        def getRuleIndex(self):
            return JetRuleParser.RULE_defineRuleSeqStmt

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterDefineRuleSeqStmt" ):
                listener.enterDefineRuleSeqStmt(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitDefineRuleSeqStmt" ):
                listener.exitDefineRuleSeqStmt(self)




    def defineRuleSeqStmt(self):

        localctx = JetRuleParser.DefineRuleSeqStmtContext(self, self._ctx, self.state)
        self.enterRule(localctx, 24, self.RULE_defineRuleSeqStmt)
        self._la = 0 # Token type
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 274
            self.match(JetRuleParser.RULESEQ)
            self.state = 275
            localctx.ruleseqName = self.match(JetRuleParser.Identifier)
            self.state = 276
            self.match(JetRuleParser.T__0)
            self.state = 280
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.COMMENT:
                self.state = 277
                self.match(JetRuleParser.COMMENT)
                self.state = 282
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 283
            self.match(JetRuleParser.MainRuleSets)
            self.state = 284
            self.match(JetRuleParser.ASSIGN)
            self.state = 285
            self.match(JetRuleParser.T__3)
            self.state = 289
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.COMMENT:
                self.state = 286
                self.match(JetRuleParser.COMMENT)
                self.state = 291
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 292
            self.ruleSetDefinitions()
            self.state = 296
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.COMMENT:
                self.state = 293
                self.match(JetRuleParser.COMMENT)
                self.state = 298
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 299
            self.match(JetRuleParser.T__4)
            self.state = 301
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            if _la==JetRuleParser.T__2:
                self.state = 300
                self.match(JetRuleParser.T__2)


            self.state = 306
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.COMMENT:
                self.state = 303
                self.match(JetRuleParser.COMMENT)
                self.state = 308
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 309
            self.match(JetRuleParser.T__1)
            self.state = 310
            self.match(JetRuleParser.SEMICOLON)
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class RuleSetDefinitionsContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.rsName = None # Token

        def STRING(self):
            return self.getToken(JetRuleParser.STRING, 0)

        def ruleSetDefinitions(self, i:int=None):
            if i is None:
                return self.getTypedRuleContexts(JetRuleParser.RuleSetDefinitionsContext)
            else:
                return self.getTypedRuleContext(JetRuleParser.RuleSetDefinitionsContext,i)


        def COMMENT(self, i:int=None):
            if i is None:
                return self.getTokens(JetRuleParser.COMMENT)
            else:
                return self.getToken(JetRuleParser.COMMENT, i)

        def getRuleIndex(self):
            return JetRuleParser.RULE_ruleSetDefinitions

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterRuleSetDefinitions" ):
                listener.enterRuleSetDefinitions(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitRuleSetDefinitions" ):
                listener.exitRuleSetDefinitions(self)




    def ruleSetDefinitions(self):

        localctx = JetRuleParser.RuleSetDefinitionsContext(self, self._ctx, self.state)
        self.enterRule(localctx, 26, self.RULE_ruleSetDefinitions)
        self._la = 0 # Token type
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 312
            localctx.rsName = self.match(JetRuleParser.STRING)
            self.state = 323
            self._errHandler.sync(self)
            _alt = self._interp.adaptivePredict(self._input,27,self._ctx)
            while _alt!=2 and _alt!=ATN.INVALID_ALT_NUMBER:
                if _alt==1:
                    self.state = 313
                    self.match(JetRuleParser.T__2)
                    self.state = 317
                    self._errHandler.sync(self)
                    _la = self._input.LA(1)
                    while _la==JetRuleParser.COMMENT:
                        self.state = 314
                        self.match(JetRuleParser.COMMENT)
                        self.state = 319
                        self._errHandler.sync(self)
                        _la = self._input.LA(1)

                    self.state = 320
                    self.ruleSetDefinitions() 
                self.state = 325
                self._errHandler.sync(self)
                _alt = self._interp.adaptivePredict(self._input,27,self._ctx)

        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class DefineLiteralStmtContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser

        def int32LiteralStmt(self):
            return self.getTypedRuleContext(JetRuleParser.Int32LiteralStmtContext,0)


        def uInt32LiteralStmt(self):
            return self.getTypedRuleContext(JetRuleParser.UInt32LiteralStmtContext,0)


        def int64LiteralStmt(self):
            return self.getTypedRuleContext(JetRuleParser.Int64LiteralStmtContext,0)


        def uInt64LiteralStmt(self):
            return self.getTypedRuleContext(JetRuleParser.UInt64LiteralStmtContext,0)


        def doubleLiteralStmt(self):
            return self.getTypedRuleContext(JetRuleParser.DoubleLiteralStmtContext,0)


        def stringLiteralStmt(self):
            return self.getTypedRuleContext(JetRuleParser.StringLiteralStmtContext,0)


        def dateLiteralStmt(self):
            return self.getTypedRuleContext(JetRuleParser.DateLiteralStmtContext,0)


        def datetimeLiteralStmt(self):
            return self.getTypedRuleContext(JetRuleParser.DatetimeLiteralStmtContext,0)


        def booleanLiteralStmt(self):
            return self.getTypedRuleContext(JetRuleParser.BooleanLiteralStmtContext,0)


        def getRuleIndex(self):
            return JetRuleParser.RULE_defineLiteralStmt

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterDefineLiteralStmt" ):
                listener.enterDefineLiteralStmt(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitDefineLiteralStmt" ):
                listener.exitDefineLiteralStmt(self)




    def defineLiteralStmt(self):

        localctx = JetRuleParser.DefineLiteralStmtContext(self, self._ctx, self.state)
        self.enterRule(localctx, 28, self.RULE_defineLiteralStmt)
        try:
            self.state = 335
            self._errHandler.sync(self)
            token = self._input.LA(1)
            if token in [JetRuleParser.Int32Type]:
                self.enterOuterAlt(localctx, 1)
                self.state = 326
                self.int32LiteralStmt()
                pass
            elif token in [JetRuleParser.UInt32Type]:
                self.enterOuterAlt(localctx, 2)
                self.state = 327
                self.uInt32LiteralStmt()
                pass
            elif token in [JetRuleParser.Int64Type]:
                self.enterOuterAlt(localctx, 3)
                self.state = 328
                self.int64LiteralStmt()
                pass
            elif token in [JetRuleParser.UInt64Type]:
                self.enterOuterAlt(localctx, 4)
                self.state = 329
                self.uInt64LiteralStmt()
                pass
            elif token in [JetRuleParser.DoubleType]:
                self.enterOuterAlt(localctx, 5)
                self.state = 330
                self.doubleLiteralStmt()
                pass
            elif token in [JetRuleParser.StringType]:
                self.enterOuterAlt(localctx, 6)
                self.state = 331
                self.stringLiteralStmt()
                pass
            elif token in [JetRuleParser.DateType]:
                self.enterOuterAlt(localctx, 7)
                self.state = 332
                self.dateLiteralStmt()
                pass
            elif token in [JetRuleParser.DatetimeType]:
                self.enterOuterAlt(localctx, 8)
                self.state = 333
                self.datetimeLiteralStmt()
                pass
            elif token in [JetRuleParser.BoolType]:
                self.enterOuterAlt(localctx, 9)
                self.state = 334
                self.booleanLiteralStmt()
                pass
            else:
                raise NoViableAltException(self)

        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class Int32LiteralStmtContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.varType = None # Token
            self.varName = None # DeclIdentifierContext
            self.declValue = None # IntExprContext

        def ASSIGN(self):
            return self.getToken(JetRuleParser.ASSIGN, 0)

        def SEMICOLON(self):
            return self.getToken(JetRuleParser.SEMICOLON, 0)

        def Int32Type(self):
            return self.getToken(JetRuleParser.Int32Type, 0)

        def declIdentifier(self):
            return self.getTypedRuleContext(JetRuleParser.DeclIdentifierContext,0)


        def intExpr(self):
            return self.getTypedRuleContext(JetRuleParser.IntExprContext,0)


        def getRuleIndex(self):
            return JetRuleParser.RULE_int32LiteralStmt

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterInt32LiteralStmt" ):
                listener.enterInt32LiteralStmt(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitInt32LiteralStmt" ):
                listener.exitInt32LiteralStmt(self)




    def int32LiteralStmt(self):

        localctx = JetRuleParser.Int32LiteralStmtContext(self, self._ctx, self.state)
        self.enterRule(localctx, 30, self.RULE_int32LiteralStmt)
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 337
            localctx.varType = self.match(JetRuleParser.Int32Type)
            self.state = 338
            localctx.varName = self.declIdentifier()
            self.state = 339
            self.match(JetRuleParser.ASSIGN)
            self.state = 340
            localctx.declValue = self.intExpr()
            self.state = 341
            self.match(JetRuleParser.SEMICOLON)
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class UInt32LiteralStmtContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.varType = None # Token
            self.varName = None # DeclIdentifierContext
            self.declValue = None # UintExprContext

        def ASSIGN(self):
            return self.getToken(JetRuleParser.ASSIGN, 0)

        def SEMICOLON(self):
            return self.getToken(JetRuleParser.SEMICOLON, 0)

        def UInt32Type(self):
            return self.getToken(JetRuleParser.UInt32Type, 0)

        def declIdentifier(self):
            return self.getTypedRuleContext(JetRuleParser.DeclIdentifierContext,0)


        def uintExpr(self):
            return self.getTypedRuleContext(JetRuleParser.UintExprContext,0)


        def getRuleIndex(self):
            return JetRuleParser.RULE_uInt32LiteralStmt

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterUInt32LiteralStmt" ):
                listener.enterUInt32LiteralStmt(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitUInt32LiteralStmt" ):
                listener.exitUInt32LiteralStmt(self)




    def uInt32LiteralStmt(self):

        localctx = JetRuleParser.UInt32LiteralStmtContext(self, self._ctx, self.state)
        self.enterRule(localctx, 32, self.RULE_uInt32LiteralStmt)
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 343
            localctx.varType = self.match(JetRuleParser.UInt32Type)
            self.state = 344
            localctx.varName = self.declIdentifier()
            self.state = 345
            self.match(JetRuleParser.ASSIGN)
            self.state = 346
            localctx.declValue = self.uintExpr()
            self.state = 347
            self.match(JetRuleParser.SEMICOLON)
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class Int64LiteralStmtContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.varType = None # Token
            self.varName = None # DeclIdentifierContext
            self.declValue = None # IntExprContext

        def ASSIGN(self):
            return self.getToken(JetRuleParser.ASSIGN, 0)

        def SEMICOLON(self):
            return self.getToken(JetRuleParser.SEMICOLON, 0)

        def Int64Type(self):
            return self.getToken(JetRuleParser.Int64Type, 0)

        def declIdentifier(self):
            return self.getTypedRuleContext(JetRuleParser.DeclIdentifierContext,0)


        def intExpr(self):
            return self.getTypedRuleContext(JetRuleParser.IntExprContext,0)


        def getRuleIndex(self):
            return JetRuleParser.RULE_int64LiteralStmt

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterInt64LiteralStmt" ):
                listener.enterInt64LiteralStmt(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitInt64LiteralStmt" ):
                listener.exitInt64LiteralStmt(self)




    def int64LiteralStmt(self):

        localctx = JetRuleParser.Int64LiteralStmtContext(self, self._ctx, self.state)
        self.enterRule(localctx, 34, self.RULE_int64LiteralStmt)
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 349
            localctx.varType = self.match(JetRuleParser.Int64Type)
            self.state = 350
            localctx.varName = self.declIdentifier()
            self.state = 351
            self.match(JetRuleParser.ASSIGN)
            self.state = 352
            localctx.declValue = self.intExpr()
            self.state = 353
            self.match(JetRuleParser.SEMICOLON)
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class UInt64LiteralStmtContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.varType = None # Token
            self.varName = None # DeclIdentifierContext
            self.declValue = None # UintExprContext

        def ASSIGN(self):
            return self.getToken(JetRuleParser.ASSIGN, 0)

        def SEMICOLON(self):
            return self.getToken(JetRuleParser.SEMICOLON, 0)

        def UInt64Type(self):
            return self.getToken(JetRuleParser.UInt64Type, 0)

        def declIdentifier(self):
            return self.getTypedRuleContext(JetRuleParser.DeclIdentifierContext,0)


        def uintExpr(self):
            return self.getTypedRuleContext(JetRuleParser.UintExprContext,0)


        def getRuleIndex(self):
            return JetRuleParser.RULE_uInt64LiteralStmt

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterUInt64LiteralStmt" ):
                listener.enterUInt64LiteralStmt(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitUInt64LiteralStmt" ):
                listener.exitUInt64LiteralStmt(self)




    def uInt64LiteralStmt(self):

        localctx = JetRuleParser.UInt64LiteralStmtContext(self, self._ctx, self.state)
        self.enterRule(localctx, 36, self.RULE_uInt64LiteralStmt)
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 355
            localctx.varType = self.match(JetRuleParser.UInt64Type)
            self.state = 356
            localctx.varName = self.declIdentifier()
            self.state = 357
            self.match(JetRuleParser.ASSIGN)
            self.state = 358
            localctx.declValue = self.uintExpr()
            self.state = 359
            self.match(JetRuleParser.SEMICOLON)
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class DoubleLiteralStmtContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.varType = None # Token
            self.varName = None # DeclIdentifierContext
            self.declValue = None # DoubleExprContext

        def ASSIGN(self):
            return self.getToken(JetRuleParser.ASSIGN, 0)

        def SEMICOLON(self):
            return self.getToken(JetRuleParser.SEMICOLON, 0)

        def DoubleType(self):
            return self.getToken(JetRuleParser.DoubleType, 0)

        def declIdentifier(self):
            return self.getTypedRuleContext(JetRuleParser.DeclIdentifierContext,0)


        def doubleExpr(self):
            return self.getTypedRuleContext(JetRuleParser.DoubleExprContext,0)


        def getRuleIndex(self):
            return JetRuleParser.RULE_doubleLiteralStmt

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterDoubleLiteralStmt" ):
                listener.enterDoubleLiteralStmt(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitDoubleLiteralStmt" ):
                listener.exitDoubleLiteralStmt(self)




    def doubleLiteralStmt(self):

        localctx = JetRuleParser.DoubleLiteralStmtContext(self, self._ctx, self.state)
        self.enterRule(localctx, 38, self.RULE_doubleLiteralStmt)
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 361
            localctx.varType = self.match(JetRuleParser.DoubleType)
            self.state = 362
            localctx.varName = self.declIdentifier()
            self.state = 363
            self.match(JetRuleParser.ASSIGN)
            self.state = 364
            localctx.declValue = self.doubleExpr()
            self.state = 365
            self.match(JetRuleParser.SEMICOLON)
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class StringLiteralStmtContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.varType = None # Token
            self.varName = None # DeclIdentifierContext
            self.declValue = None # Token

        def ASSIGN(self):
            return self.getToken(JetRuleParser.ASSIGN, 0)

        def SEMICOLON(self):
            return self.getToken(JetRuleParser.SEMICOLON, 0)

        def StringType(self):
            return self.getToken(JetRuleParser.StringType, 0)

        def declIdentifier(self):
            return self.getTypedRuleContext(JetRuleParser.DeclIdentifierContext,0)


        def STRING(self):
            return self.getToken(JetRuleParser.STRING, 0)

        def getRuleIndex(self):
            return JetRuleParser.RULE_stringLiteralStmt

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterStringLiteralStmt" ):
                listener.enterStringLiteralStmt(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitStringLiteralStmt" ):
                listener.exitStringLiteralStmt(self)




    def stringLiteralStmt(self):

        localctx = JetRuleParser.StringLiteralStmtContext(self, self._ctx, self.state)
        self.enterRule(localctx, 40, self.RULE_stringLiteralStmt)
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 367
            localctx.varType = self.match(JetRuleParser.StringType)
            self.state = 368
            localctx.varName = self.declIdentifier()
            self.state = 369
            self.match(JetRuleParser.ASSIGN)
            self.state = 370
            localctx.declValue = self.match(JetRuleParser.STRING)
            self.state = 371
            self.match(JetRuleParser.SEMICOLON)
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class DateLiteralStmtContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.varType = None # Token
            self.varName = None # DeclIdentifierContext
            self.declValue = None # Token

        def ASSIGN(self):
            return self.getToken(JetRuleParser.ASSIGN, 0)

        def SEMICOLON(self):
            return self.getToken(JetRuleParser.SEMICOLON, 0)

        def DateType(self):
            return self.getToken(JetRuleParser.DateType, 0)

        def declIdentifier(self):
            return self.getTypedRuleContext(JetRuleParser.DeclIdentifierContext,0)


        def STRING(self):
            return self.getToken(JetRuleParser.STRING, 0)

        def getRuleIndex(self):
            return JetRuleParser.RULE_dateLiteralStmt

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterDateLiteralStmt" ):
                listener.enterDateLiteralStmt(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitDateLiteralStmt" ):
                listener.exitDateLiteralStmt(self)




    def dateLiteralStmt(self):

        localctx = JetRuleParser.DateLiteralStmtContext(self, self._ctx, self.state)
        self.enterRule(localctx, 42, self.RULE_dateLiteralStmt)
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 373
            localctx.varType = self.match(JetRuleParser.DateType)
            self.state = 374
            localctx.varName = self.declIdentifier()
            self.state = 375
            self.match(JetRuleParser.ASSIGN)
            self.state = 376
            localctx.declValue = self.match(JetRuleParser.STRING)
            self.state = 377
            self.match(JetRuleParser.SEMICOLON)
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class DatetimeLiteralStmtContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.varType = None # Token
            self.varName = None # DeclIdentifierContext
            self.declValue = None # Token

        def ASSIGN(self):
            return self.getToken(JetRuleParser.ASSIGN, 0)

        def SEMICOLON(self):
            return self.getToken(JetRuleParser.SEMICOLON, 0)

        def DatetimeType(self):
            return self.getToken(JetRuleParser.DatetimeType, 0)

        def declIdentifier(self):
            return self.getTypedRuleContext(JetRuleParser.DeclIdentifierContext,0)


        def STRING(self):
            return self.getToken(JetRuleParser.STRING, 0)

        def getRuleIndex(self):
            return JetRuleParser.RULE_datetimeLiteralStmt

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterDatetimeLiteralStmt" ):
                listener.enterDatetimeLiteralStmt(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitDatetimeLiteralStmt" ):
                listener.exitDatetimeLiteralStmt(self)




    def datetimeLiteralStmt(self):

        localctx = JetRuleParser.DatetimeLiteralStmtContext(self, self._ctx, self.state)
        self.enterRule(localctx, 44, self.RULE_datetimeLiteralStmt)
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 379
            localctx.varType = self.match(JetRuleParser.DatetimeType)
            self.state = 380
            localctx.varName = self.declIdentifier()
            self.state = 381
            self.match(JetRuleParser.ASSIGN)
            self.state = 382
            localctx.declValue = self.match(JetRuleParser.STRING)
            self.state = 383
            self.match(JetRuleParser.SEMICOLON)
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class BooleanLiteralStmtContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.varType = None # Token
            self.varName = None # DeclIdentifierContext
            self.declValue = None # Token

        def ASSIGN(self):
            return self.getToken(JetRuleParser.ASSIGN, 0)

        def SEMICOLON(self):
            return self.getToken(JetRuleParser.SEMICOLON, 0)

        def BoolType(self):
            return self.getToken(JetRuleParser.BoolType, 0)

        def declIdentifier(self):
            return self.getTypedRuleContext(JetRuleParser.DeclIdentifierContext,0)


        def STRING(self):
            return self.getToken(JetRuleParser.STRING, 0)

        def getRuleIndex(self):
            return JetRuleParser.RULE_booleanLiteralStmt

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterBooleanLiteralStmt" ):
                listener.enterBooleanLiteralStmt(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitBooleanLiteralStmt" ):
                listener.exitBooleanLiteralStmt(self)




    def booleanLiteralStmt(self):

        localctx = JetRuleParser.BooleanLiteralStmtContext(self, self._ctx, self.state)
        self.enterRule(localctx, 46, self.RULE_booleanLiteralStmt)
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 385
            localctx.varType = self.match(JetRuleParser.BoolType)
            self.state = 386
            localctx.varName = self.declIdentifier()
            self.state = 387
            self.match(JetRuleParser.ASSIGN)
            self.state = 388
            localctx.declValue = self.match(JetRuleParser.STRING)
            self.state = 389
            self.match(JetRuleParser.SEMICOLON)
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class IntExprContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser

        def PLUS(self):
            return self.getToken(JetRuleParser.PLUS, 0)

        def intExpr(self):
            return self.getTypedRuleContext(JetRuleParser.IntExprContext,0)


        def MINUS(self):
            return self.getToken(JetRuleParser.MINUS, 0)

        def DIGITS(self):
            return self.getToken(JetRuleParser.DIGITS, 0)

        def getRuleIndex(self):
            return JetRuleParser.RULE_intExpr

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterIntExpr" ):
                listener.enterIntExpr(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitIntExpr" ):
                listener.exitIntExpr(self)




    def intExpr(self):

        localctx = JetRuleParser.IntExprContext(self, self._ctx, self.state)
        self.enterRule(localctx, 48, self.RULE_intExpr)
        try:
            self.state = 396
            self._errHandler.sync(self)
            token = self._input.LA(1)
            if token in [JetRuleParser.PLUS]:
                self.enterOuterAlt(localctx, 1)
                self.state = 391
                self.match(JetRuleParser.PLUS)
                self.state = 392
                self.intExpr()
                pass
            elif token in [JetRuleParser.MINUS]:
                self.enterOuterAlt(localctx, 2)
                self.state = 393
                self.match(JetRuleParser.MINUS)
                self.state = 394
                self.intExpr()
                pass
            elif token in [JetRuleParser.DIGITS]:
                self.enterOuterAlt(localctx, 3)
                self.state = 395
                self.match(JetRuleParser.DIGITS)
                pass
            else:
                raise NoViableAltException(self)

        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class UintExprContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser

        def PLUS(self):
            return self.getToken(JetRuleParser.PLUS, 0)

        def uintExpr(self):
            return self.getTypedRuleContext(JetRuleParser.UintExprContext,0)


        def DIGITS(self):
            return self.getToken(JetRuleParser.DIGITS, 0)

        def getRuleIndex(self):
            return JetRuleParser.RULE_uintExpr

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterUintExpr" ):
                listener.enterUintExpr(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitUintExpr" ):
                listener.exitUintExpr(self)




    def uintExpr(self):

        localctx = JetRuleParser.UintExprContext(self, self._ctx, self.state)
        self.enterRule(localctx, 50, self.RULE_uintExpr)
        try:
            self.state = 401
            self._errHandler.sync(self)
            token = self._input.LA(1)
            if token in [JetRuleParser.PLUS]:
                self.enterOuterAlt(localctx, 1)
                self.state = 398
                self.match(JetRuleParser.PLUS)
                self.state = 399
                self.uintExpr()
                pass
            elif token in [JetRuleParser.DIGITS]:
                self.enterOuterAlt(localctx, 2)
                self.state = 400
                self.match(JetRuleParser.DIGITS)
                pass
            else:
                raise NoViableAltException(self)

        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class DoubleExprContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser

        def PLUS(self):
            return self.getToken(JetRuleParser.PLUS, 0)

        def doubleExpr(self):
            return self.getTypedRuleContext(JetRuleParser.DoubleExprContext,0)


        def MINUS(self):
            return self.getToken(JetRuleParser.MINUS, 0)

        def DIGITS(self, i:int=None):
            if i is None:
                return self.getTokens(JetRuleParser.DIGITS)
            else:
                return self.getToken(JetRuleParser.DIGITS, i)

        def getRuleIndex(self):
            return JetRuleParser.RULE_doubleExpr

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterDoubleExpr" ):
                listener.enterDoubleExpr(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitDoubleExpr" ):
                listener.exitDoubleExpr(self)




    def doubleExpr(self):

        localctx = JetRuleParser.DoubleExprContext(self, self._ctx, self.state)
        self.enterRule(localctx, 52, self.RULE_doubleExpr)
        try:
            self.state = 412
            self._errHandler.sync(self)
            token = self._input.LA(1)
            if token in [JetRuleParser.PLUS]:
                self.enterOuterAlt(localctx, 1)
                self.state = 403
                self.match(JetRuleParser.PLUS)
                self.state = 404
                self.doubleExpr()
                pass
            elif token in [JetRuleParser.MINUS]:
                self.enterOuterAlt(localctx, 2)
                self.state = 405
                self.match(JetRuleParser.MINUS)
                self.state = 406
                self.doubleExpr()
                pass
            elif token in [JetRuleParser.DIGITS]:
                self.enterOuterAlt(localctx, 3)
                self.state = 407
                self.match(JetRuleParser.DIGITS)
                self.state = 410
                self._errHandler.sync(self)
                la_ = self._interp.adaptivePredict(self._input,31,self._ctx)
                if la_ == 1:
                    self.state = 408
                    self.match(JetRuleParser.T__6)
                    self.state = 409
                    self.match(JetRuleParser.DIGITS)


                pass
            else:
                raise NoViableAltException(self)

        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class DeclIdentifierContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser

        def Identifier(self, i:int=None):
            if i is None:
                return self.getTokens(JetRuleParser.Identifier)
            else:
                return self.getToken(JetRuleParser.Identifier, i)

        def STRING(self):
            return self.getToken(JetRuleParser.STRING, 0)

        def getRuleIndex(self):
            return JetRuleParser.RULE_declIdentifier

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterDeclIdentifier" ):
                listener.enterDeclIdentifier(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitDeclIdentifier" ):
                listener.exitDeclIdentifier(self)




    def declIdentifier(self):

        localctx = JetRuleParser.DeclIdentifierContext(self, self._ctx, self.state)
        self.enterRule(localctx, 54, self.RULE_declIdentifier)
        try:
            self.state = 421
            self._errHandler.sync(self)
            la_ = self._interp.adaptivePredict(self._input,33,self._ctx)
            if la_ == 1:
                self.enterOuterAlt(localctx, 1)
                self.state = 414
                self.match(JetRuleParser.Identifier)
                self.state = 415
                self.match(JetRuleParser.T__7)
                self.state = 416
                self.match(JetRuleParser.Identifier)
                pass

            elif la_ == 2:
                self.enterOuterAlt(localctx, 2)
                self.state = 417
                self.match(JetRuleParser.Identifier)
                self.state = 418
                self.match(JetRuleParser.T__7)
                self.state = 419
                self.match(JetRuleParser.STRING)
                pass

            elif la_ == 3:
                self.enterOuterAlt(localctx, 3)
                self.state = 420
                self.match(JetRuleParser.Identifier)
                pass


        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class DefineResourceStmtContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser

        def namedResourceStmt(self):
            return self.getTypedRuleContext(JetRuleParser.NamedResourceStmtContext,0)


        def volatileResourceStmt(self):
            return self.getTypedRuleContext(JetRuleParser.VolatileResourceStmtContext,0)


        def getRuleIndex(self):
            return JetRuleParser.RULE_defineResourceStmt

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterDefineResourceStmt" ):
                listener.enterDefineResourceStmt(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitDefineResourceStmt" ):
                listener.exitDefineResourceStmt(self)




    def defineResourceStmt(self):

        localctx = JetRuleParser.DefineResourceStmtContext(self, self._ctx, self.state)
        self.enterRule(localctx, 56, self.RULE_defineResourceStmt)
        try:
            self.state = 425
            self._errHandler.sync(self)
            token = self._input.LA(1)
            if token in [JetRuleParser.ResourceType]:
                self.enterOuterAlt(localctx, 1)
                self.state = 423
                self.namedResourceStmt()
                pass
            elif token in [JetRuleParser.VolatileResourceType]:
                self.enterOuterAlt(localctx, 2)
                self.state = 424
                self.volatileResourceStmt()
                pass
            else:
                raise NoViableAltException(self)

        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class NamedResourceStmtContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.resName = None # DeclIdentifierContext
            self.resCtx = None # ResourceValueContext

        def ResourceType(self):
            return self.getToken(JetRuleParser.ResourceType, 0)

        def ASSIGN(self):
            return self.getToken(JetRuleParser.ASSIGN, 0)

        def SEMICOLON(self):
            return self.getToken(JetRuleParser.SEMICOLON, 0)

        def declIdentifier(self):
            return self.getTypedRuleContext(JetRuleParser.DeclIdentifierContext,0)


        def resourceValue(self):
            return self.getTypedRuleContext(JetRuleParser.ResourceValueContext,0)


        def getRuleIndex(self):
            return JetRuleParser.RULE_namedResourceStmt

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterNamedResourceStmt" ):
                listener.enterNamedResourceStmt(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitNamedResourceStmt" ):
                listener.exitNamedResourceStmt(self)




    def namedResourceStmt(self):

        localctx = JetRuleParser.NamedResourceStmtContext(self, self._ctx, self.state)
        self.enterRule(localctx, 58, self.RULE_namedResourceStmt)
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 427
            self.match(JetRuleParser.ResourceType)
            self.state = 428
            localctx.resName = self.declIdentifier()
            self.state = 429
            self.match(JetRuleParser.ASSIGN)
            self.state = 430
            localctx.resCtx = self.resourceValue()
            self.state = 431
            self.match(JetRuleParser.SEMICOLON)
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class VolatileResourceStmtContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.resType = None # Token
            self.resName = None # DeclIdentifierContext
            self.resVal = None # Token

        def ASSIGN(self):
            return self.getToken(JetRuleParser.ASSIGN, 0)

        def SEMICOLON(self):
            return self.getToken(JetRuleParser.SEMICOLON, 0)

        def VolatileResourceType(self):
            return self.getToken(JetRuleParser.VolatileResourceType, 0)

        def declIdentifier(self):
            return self.getTypedRuleContext(JetRuleParser.DeclIdentifierContext,0)


        def STRING(self):
            return self.getToken(JetRuleParser.STRING, 0)

        def getRuleIndex(self):
            return JetRuleParser.RULE_volatileResourceStmt

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterVolatileResourceStmt" ):
                listener.enterVolatileResourceStmt(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitVolatileResourceStmt" ):
                listener.exitVolatileResourceStmt(self)




    def volatileResourceStmt(self):

        localctx = JetRuleParser.VolatileResourceStmtContext(self, self._ctx, self.state)
        self.enterRule(localctx, 60, self.RULE_volatileResourceStmt)
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 433
            localctx.resType = self.match(JetRuleParser.VolatileResourceType)
            self.state = 434
            localctx.resName = self.declIdentifier()
            self.state = 435
            self.match(JetRuleParser.ASSIGN)
            self.state = 436
            localctx.resVal = self.match(JetRuleParser.STRING)
            self.state = 437
            self.match(JetRuleParser.SEMICOLON)
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class ResourceValueContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.kws = None # KeywordsContext
            self.resVal = None # Token

        def keywords(self):
            return self.getTypedRuleContext(JetRuleParser.KeywordsContext,0)


        def CreateUUIDResource(self):
            return self.getToken(JetRuleParser.CreateUUIDResource, 0)

        def STRING(self):
            return self.getToken(JetRuleParser.STRING, 0)

        def getRuleIndex(self):
            return JetRuleParser.RULE_resourceValue

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterResourceValue" ):
                listener.enterResourceValue(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitResourceValue" ):
                listener.exitResourceValue(self)




    def resourceValue(self):

        localctx = JetRuleParser.ResourceValueContext(self, self._ctx, self.state)
        self.enterRule(localctx, 62, self.RULE_resourceValue)
        try:
            self.state = 442
            self._errHandler.sync(self)
            token = self._input.LA(1)
            if token in [JetRuleParser.TRUE, JetRuleParser.FALSE, JetRuleParser.NULL]:
                self.enterOuterAlt(localctx, 1)
                self.state = 439
                localctx.kws = self.keywords()
                pass
            elif token in [JetRuleParser.CreateUUIDResource]:
                self.enterOuterAlt(localctx, 2)
                self.state = 440
                localctx.resVal = self.match(JetRuleParser.CreateUUIDResource)
                pass
            elif token in [JetRuleParser.STRING]:
                self.enterOuterAlt(localctx, 3)
                self.state = 441
                localctx.resVal = self.match(JetRuleParser.STRING)
                pass
            else:
                raise NoViableAltException(self)

        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class LookupTableStmtContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.lookupName = None # DeclIdentifierContext
            self.tblKeys = None # StringListContext

        def LookupTable(self):
            return self.getToken(JetRuleParser.LookupTable, 0)

        def csvLocation(self):
            return self.getTypedRuleContext(JetRuleParser.CsvLocationContext,0)


        def Key(self):
            return self.getToken(JetRuleParser.Key, 0)

        def ASSIGN(self, i:int=None):
            if i is None:
                return self.getTokens(JetRuleParser.ASSIGN)
            else:
                return self.getToken(JetRuleParser.ASSIGN, i)

        def Columns(self):
            return self.getToken(JetRuleParser.Columns, 0)

        def columnDefinitions(self):
            return self.getTypedRuleContext(JetRuleParser.ColumnDefinitionsContext,0)


        def SEMICOLON(self):
            return self.getToken(JetRuleParser.SEMICOLON, 0)

        def declIdentifier(self):
            return self.getTypedRuleContext(JetRuleParser.DeclIdentifierContext,0)


        def stringList(self):
            return self.getTypedRuleContext(JetRuleParser.StringListContext,0)


        def COMMENT(self, i:int=None):
            if i is None:
                return self.getTokens(JetRuleParser.COMMENT)
            else:
                return self.getToken(JetRuleParser.COMMENT, i)

        def getRuleIndex(self):
            return JetRuleParser.RULE_lookupTableStmt

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterLookupTableStmt" ):
                listener.enterLookupTableStmt(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitLookupTableStmt" ):
                listener.exitLookupTableStmt(self)




    def lookupTableStmt(self):

        localctx = JetRuleParser.LookupTableStmtContext(self, self._ctx, self.state)
        self.enterRule(localctx, 64, self.RULE_lookupTableStmt)
        self._la = 0 # Token type
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 444
            self.match(JetRuleParser.LookupTable)
            self.state = 445
            localctx.lookupName = self.declIdentifier()
            self.state = 446
            self.match(JetRuleParser.T__0)
            self.state = 450
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.COMMENT:
                self.state = 447
                self.match(JetRuleParser.COMMENT)
                self.state = 452
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 453
            self.csvLocation()
            self.state = 457
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.COMMENT:
                self.state = 454
                self.match(JetRuleParser.COMMENT)
                self.state = 459
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 460
            self.match(JetRuleParser.Key)
            self.state = 461
            self.match(JetRuleParser.ASSIGN)
            self.state = 462
            localctx.tblKeys = self.stringList()
            self.state = 463
            self.match(JetRuleParser.T__2)
            self.state = 467
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.COMMENT:
                self.state = 464
                self.match(JetRuleParser.COMMENT)
                self.state = 469
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 470
            self.match(JetRuleParser.Columns)
            self.state = 471
            self.match(JetRuleParser.ASSIGN)
            self.state = 472
            self.match(JetRuleParser.T__3)
            self.state = 476
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.COMMENT:
                self.state = 473
                self.match(JetRuleParser.COMMENT)
                self.state = 478
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 479
            self.columnDefinitions()
            self.state = 483
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.COMMENT:
                self.state = 480
                self.match(JetRuleParser.COMMENT)
                self.state = 485
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 486
            self.match(JetRuleParser.T__4)
            self.state = 488
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            if _la==JetRuleParser.T__2:
                self.state = 487
                self.match(JetRuleParser.T__2)


            self.state = 493
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.COMMENT:
                self.state = 490
                self.match(JetRuleParser.COMMENT)
                self.state = 495
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 496
            self.match(JetRuleParser.T__1)
            self.state = 497
            self.match(JetRuleParser.SEMICOLON)
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class CsvLocationContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.tblStorageName = None # Token
            self.csvFileName = None # Token

        def TableName(self):
            return self.getToken(JetRuleParser.TableName, 0)

        def ASSIGN(self):
            return self.getToken(JetRuleParser.ASSIGN, 0)

        def Identifier(self):
            return self.getToken(JetRuleParser.Identifier, 0)

        def CSVFileName(self):
            return self.getToken(JetRuleParser.CSVFileName, 0)

        def STRING(self):
            return self.getToken(JetRuleParser.STRING, 0)

        def getRuleIndex(self):
            return JetRuleParser.RULE_csvLocation

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterCsvLocation" ):
                listener.enterCsvLocation(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitCsvLocation" ):
                listener.exitCsvLocation(self)




    def csvLocation(self):

        localctx = JetRuleParser.CsvLocationContext(self, self._ctx, self.state)
        self.enterRule(localctx, 66, self.RULE_csvLocation)
        try:
            self.state = 507
            self._errHandler.sync(self)
            token = self._input.LA(1)
            if token in [JetRuleParser.TableName]:
                self.enterOuterAlt(localctx, 1)
                self.state = 499
                self.match(JetRuleParser.TableName)
                self.state = 500
                self.match(JetRuleParser.ASSIGN)
                self.state = 501
                localctx.tblStorageName = self.match(JetRuleParser.Identifier)
                self.state = 502
                self.match(JetRuleParser.T__2)
                pass
            elif token in [JetRuleParser.CSVFileName]:
                self.enterOuterAlt(localctx, 2)
                self.state = 503
                self.match(JetRuleParser.CSVFileName)
                self.state = 504
                self.match(JetRuleParser.ASSIGN)
                self.state = 505
                localctx.csvFileName = self.match(JetRuleParser.STRING)
                self.state = 506
                self.match(JetRuleParser.T__2)
                pass
            else:
                raise NoViableAltException(self)

        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class StringListContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.seqCtx = None # StringSeqContext

        def stringSeq(self):
            return self.getTypedRuleContext(JetRuleParser.StringSeqContext,0)


        def getRuleIndex(self):
            return JetRuleParser.RULE_stringList

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterStringList" ):
                listener.enterStringList(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitStringList" ):
                listener.exitStringList(self)




    def stringList(self):

        localctx = JetRuleParser.StringListContext(self, self._ctx, self.state)
        self.enterRule(localctx, 68, self.RULE_stringList)
        self._la = 0 # Token type
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 509
            self.match(JetRuleParser.T__3)
            self.state = 511
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            if _la==JetRuleParser.STRING:
                self.state = 510
                localctx.seqCtx = self.stringSeq()


            self.state = 513
            self.match(JetRuleParser.T__4)
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class StringSeqContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self._STRING = None # Token
            self.slist = list() # of Tokens

        def STRING(self, i:int=None):
            if i is None:
                return self.getTokens(JetRuleParser.STRING)
            else:
                return self.getToken(JetRuleParser.STRING, i)

        def getRuleIndex(self):
            return JetRuleParser.RULE_stringSeq

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterStringSeq" ):
                listener.enterStringSeq(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitStringSeq" ):
                listener.exitStringSeq(self)




    def stringSeq(self):

        localctx = JetRuleParser.StringSeqContext(self, self._ctx, self.state)
        self.enterRule(localctx, 70, self.RULE_stringSeq)
        self._la = 0 # Token type
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 515
            localctx._STRING = self.match(JetRuleParser.STRING)
            localctx.slist.append(localctx._STRING)
            self.state = 520
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.T__2:
                self.state = 516
                self.match(JetRuleParser.T__2)
                self.state = 517
                localctx._STRING = self.match(JetRuleParser.STRING)
                localctx.slist.append(localctx._STRING)
                self.state = 522
                self._errHandler.sync(self)
                _la = self._input.LA(1)

        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class ColumnDefinitionsContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.columnName = None # Token
            self.array = None # Token
            self.columnType = None # DataPropertyTypeContext

        def STRING(self):
            return self.getToken(JetRuleParser.STRING, 0)

        def dataPropertyType(self):
            return self.getTypedRuleContext(JetRuleParser.DataPropertyTypeContext,0)


        def columnDefinitions(self, i:int=None):
            if i is None:
                return self.getTypedRuleContexts(JetRuleParser.ColumnDefinitionsContext)
            else:
                return self.getTypedRuleContext(JetRuleParser.ColumnDefinitionsContext,i)


        def ARRAY(self):
            return self.getToken(JetRuleParser.ARRAY, 0)

        def COMMENT(self, i:int=None):
            if i is None:
                return self.getTokens(JetRuleParser.COMMENT)
            else:
                return self.getToken(JetRuleParser.COMMENT, i)

        def getRuleIndex(self):
            return JetRuleParser.RULE_columnDefinitions

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterColumnDefinitions" ):
                listener.enterColumnDefinitions(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitColumnDefinitions" ):
                listener.exitColumnDefinitions(self)




    def columnDefinitions(self):

        localctx = JetRuleParser.ColumnDefinitionsContext(self, self._ctx, self.state)
        self.enterRule(localctx, 72, self.RULE_columnDefinitions)
        self._la = 0 # Token type
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 523
            localctx.columnName = self.match(JetRuleParser.STRING)
            self.state = 524
            self.match(JetRuleParser.T__5)
            self.state = 526
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            if _la==JetRuleParser.ARRAY:
                self.state = 525
                localctx.array = self.match(JetRuleParser.ARRAY)


            self.state = 528
            localctx.columnType = self.dataPropertyType()
            self.state = 539
            self._errHandler.sync(self)
            _alt = self._interp.adaptivePredict(self._input,48,self._ctx)
            while _alt!=2 and _alt!=ATN.INVALID_ALT_NUMBER:
                if _alt==1:
                    self.state = 529
                    self.match(JetRuleParser.T__2)
                    self.state = 533
                    self._errHandler.sync(self)
                    _la = self._input.LA(1)
                    while _la==JetRuleParser.COMMENT:
                        self.state = 530
                        self.match(JetRuleParser.COMMENT)
                        self.state = 535
                        self._errHandler.sync(self)
                        _la = self._input.LA(1)

                    self.state = 536
                    self.columnDefinitions() 
                self.state = 541
                self._errHandler.sync(self)
                _alt = self._interp.adaptivePredict(self._input,48,self._ctx)

        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class JetRuleStmtContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.ruleName = None # Token

        def SEMICOLON(self):
            return self.getToken(JetRuleParser.SEMICOLON, 0)

        def Identifier(self):
            return self.getToken(JetRuleParser.Identifier, 0)

        def ruleProperties(self, i:int=None):
            if i is None:
                return self.getTypedRuleContexts(JetRuleParser.RulePropertiesContext)
            else:
                return self.getTypedRuleContext(JetRuleParser.RulePropertiesContext,i)


        def COMMENT(self, i:int=None):
            if i is None:
                return self.getTokens(JetRuleParser.COMMENT)
            else:
                return self.getToken(JetRuleParser.COMMENT, i)

        def antecedent(self, i:int=None):
            if i is None:
                return self.getTypedRuleContexts(JetRuleParser.AntecedentContext)
            else:
                return self.getTypedRuleContext(JetRuleParser.AntecedentContext,i)


        def consequent(self, i:int=None):
            if i is None:
                return self.getTypedRuleContexts(JetRuleParser.ConsequentContext)
            else:
                return self.getTypedRuleContext(JetRuleParser.ConsequentContext,i)


        def getRuleIndex(self):
            return JetRuleParser.RULE_jetRuleStmt

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterJetRuleStmt" ):
                listener.enterJetRuleStmt(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitJetRuleStmt" ):
                listener.exitJetRuleStmt(self)




    def jetRuleStmt(self):

        localctx = JetRuleParser.JetRuleStmtContext(self, self._ctx, self.state)
        self.enterRule(localctx, 74, self.RULE_jetRuleStmt)
        self._la = 0 # Token type
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 542
            self.match(JetRuleParser.T__3)
            self.state = 543
            localctx.ruleName = self.match(JetRuleParser.Identifier)
            self.state = 547
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.T__2:
                self.state = 544
                self.ruleProperties()
                self.state = 549
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 550
            self.match(JetRuleParser.T__4)
            self.state = 551
            self.match(JetRuleParser.T__7)
            self.state = 555
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.COMMENT:
                self.state = 552
                self.match(JetRuleParser.COMMENT)
                self.state = 557
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 565 
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while True:
                self.state = 558
                self.antecedent()
                self.state = 562
                self._errHandler.sync(self)
                _la = self._input.LA(1)
                while _la==JetRuleParser.COMMENT:
                    self.state = 559
                    self.match(JetRuleParser.COMMENT)
                    self.state = 564
                    self._errHandler.sync(self)
                    _la = self._input.LA(1)

                self.state = 567 
                self._errHandler.sync(self)
                _la = self._input.LA(1)
                if not (_la==JetRuleParser.T__9 or _la==JetRuleParser.NOT):
                    break

            self.state = 569
            self.match(JetRuleParser.T__8)
            self.state = 573
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while _la==JetRuleParser.COMMENT:
                self.state = 570
                self.match(JetRuleParser.COMMENT)
                self.state = 575
                self._errHandler.sync(self)
                _la = self._input.LA(1)

            self.state = 583 
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            while True:
                self.state = 576
                self.consequent()
                self.state = 580
                self._errHandler.sync(self)
                _la = self._input.LA(1)
                while _la==JetRuleParser.COMMENT:
                    self.state = 577
                    self.match(JetRuleParser.COMMENT)
                    self.state = 582
                    self._errHandler.sync(self)
                    _la = self._input.LA(1)

                self.state = 585 
                self._errHandler.sync(self)
                _la = self._input.LA(1)
                if not (_la==JetRuleParser.T__9):
                    break

            self.state = 587
            self.match(JetRuleParser.SEMICOLON)
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class RulePropertiesContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.key = None # Token
            self.valCtx = None # PropertyValueContext

        def ASSIGN(self):
            return self.getToken(JetRuleParser.ASSIGN, 0)

        def Identifier(self):
            return self.getToken(JetRuleParser.Identifier, 0)

        def propertyValue(self):
            return self.getTypedRuleContext(JetRuleParser.PropertyValueContext,0)


        def getRuleIndex(self):
            return JetRuleParser.RULE_ruleProperties

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterRuleProperties" ):
                listener.enterRuleProperties(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitRuleProperties" ):
                listener.exitRuleProperties(self)




    def ruleProperties(self):

        localctx = JetRuleParser.RulePropertiesContext(self, self._ctx, self.state)
        self.enterRule(localctx, 76, self.RULE_ruleProperties)
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 589
            self.match(JetRuleParser.T__2)
            self.state = 590
            localctx.key = self.match(JetRuleParser.Identifier)
            self.state = 591
            self.match(JetRuleParser.ASSIGN)
            self.state = 592
            localctx.valCtx = self.propertyValue()
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class PropertyValueContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.val = None # Token
            self.intval = None # IntExprContext

        def STRING(self):
            return self.getToken(JetRuleParser.STRING, 0)

        def TRUE(self):
            return self.getToken(JetRuleParser.TRUE, 0)

        def FALSE(self):
            return self.getToken(JetRuleParser.FALSE, 0)

        def intExpr(self):
            return self.getTypedRuleContext(JetRuleParser.IntExprContext,0)


        def getRuleIndex(self):
            return JetRuleParser.RULE_propertyValue

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterPropertyValue" ):
                listener.enterPropertyValue(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitPropertyValue" ):
                listener.exitPropertyValue(self)




    def propertyValue(self):

        localctx = JetRuleParser.PropertyValueContext(self, self._ctx, self.state)
        self.enterRule(localctx, 78, self.RULE_propertyValue)
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 598
            self._errHandler.sync(self)
            token = self._input.LA(1)
            if token in [JetRuleParser.STRING]:
                self.state = 594
                localctx.val = self.match(JetRuleParser.STRING)
                pass
            elif token in [JetRuleParser.TRUE]:
                self.state = 595
                localctx.val = self.match(JetRuleParser.TRUE)
                pass
            elif token in [JetRuleParser.FALSE]:
                self.state = 596
                localctx.val = self.match(JetRuleParser.FALSE)
                pass
            elif token in [JetRuleParser.PLUS, JetRuleParser.MINUS, JetRuleParser.DIGITS]:
                self.state = 597
                localctx.intval = self.intExpr()
                pass
            else:
                raise NoViableAltException(self)

        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class AntecedentContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.n = None # Token
            self.s = None # AtomContext
            self.p = None # AtomContext
            self.o = None # ObjectAtomContext
            self.f = None # ExprTermContext

        def atom(self, i:int=None):
            if i is None:
                return self.getTypedRuleContexts(JetRuleParser.AtomContext)
            else:
                return self.getTypedRuleContext(JetRuleParser.AtomContext,i)


        def objectAtom(self):
            return self.getTypedRuleContext(JetRuleParser.ObjectAtomContext,0)


        def NOT(self):
            return self.getToken(JetRuleParser.NOT, 0)

        def exprTerm(self):
            return self.getTypedRuleContext(JetRuleParser.ExprTermContext,0)


        def getRuleIndex(self):
            return JetRuleParser.RULE_antecedent

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterAntecedent" ):
                listener.enterAntecedent(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitAntecedent" ):
                listener.exitAntecedent(self)




    def antecedent(self):

        localctx = JetRuleParser.AntecedentContext(self, self._ctx, self.state)
        self.enterRule(localctx, 80, self.RULE_antecedent)
        self._la = 0 # Token type
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 601
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            if _la==JetRuleParser.NOT:
                self.state = 600
                localctx.n = self.match(JetRuleParser.NOT)


            self.state = 603
            self.match(JetRuleParser.T__9)
            self.state = 604
            localctx.s = self.atom()
            self.state = 605
            localctx.p = self.atom()
            self.state = 606
            localctx.o = self.objectAtom()
            self.state = 607
            self.match(JetRuleParser.T__10)
            self.state = 609
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            if _la==JetRuleParser.T__6:
                self.state = 608
                self.match(JetRuleParser.T__6)


            self.state = 617
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            if _la==JetRuleParser.T__3:
                self.state = 611
                self.match(JetRuleParser.T__3)
                self.state = 612
                localctx.f = self.exprTerm(0)
                self.state = 613
                self.match(JetRuleParser.T__4)
                self.state = 615
                self._errHandler.sync(self)
                _la = self._input.LA(1)
                if _la==JetRuleParser.T__6:
                    self.state = 614
                    self.match(JetRuleParser.T__6)




        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class ConsequentContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.s = None # AtomContext
            self.p = None # AtomContext
            self.o = None # ExprTermContext

        def atom(self, i:int=None):
            if i is None:
                return self.getTypedRuleContexts(JetRuleParser.AtomContext)
            else:
                return self.getTypedRuleContext(JetRuleParser.AtomContext,i)


        def exprTerm(self):
            return self.getTypedRuleContext(JetRuleParser.ExprTermContext,0)


        def getRuleIndex(self):
            return JetRuleParser.RULE_consequent

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterConsequent" ):
                listener.enterConsequent(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitConsequent" ):
                listener.exitConsequent(self)




    def consequent(self):

        localctx = JetRuleParser.ConsequentContext(self, self._ctx, self.state)
        self.enterRule(localctx, 82, self.RULE_consequent)
        self._la = 0 # Token type
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 619
            self.match(JetRuleParser.T__9)
            self.state = 620
            localctx.s = self.atom()
            self.state = 621
            localctx.p = self.atom()
            self.state = 622
            localctx.o = self.exprTerm(0)
            self.state = 623
            self.match(JetRuleParser.T__10)
            self.state = 625
            self._errHandler.sync(self)
            _la = self._input.LA(1)
            if _la==JetRuleParser.T__6:
                self.state = 624
                self.match(JetRuleParser.T__6)


        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class AtomContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser

        def Identifier(self):
            return self.getToken(JetRuleParser.Identifier, 0)

        def declIdentifier(self):
            return self.getTypedRuleContext(JetRuleParser.DeclIdentifierContext,0)


        def getRuleIndex(self):
            return JetRuleParser.RULE_atom

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterAtom" ):
                listener.enterAtom(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitAtom" ):
                listener.exitAtom(self)




    def atom(self):

        localctx = JetRuleParser.AtomContext(self, self._ctx, self.state)
        self.enterRule(localctx, 84, self.RULE_atom)
        try:
            self.state = 630
            self._errHandler.sync(self)
            token = self._input.LA(1)
            if token in [JetRuleParser.T__11]:
                self.enterOuterAlt(localctx, 1)
                self.state = 627
                self.match(JetRuleParser.T__11)
                self.state = 628
                self.match(JetRuleParser.Identifier)
                pass
            elif token in [JetRuleParser.Identifier]:
                self.enterOuterAlt(localctx, 2)
                self.state = 629
                self.declIdentifier()
                pass
            else:
                raise NoViableAltException(self)

        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class ObjectAtomContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.kws = None # KeywordsContext

        def atom(self):
            return self.getTypedRuleContext(JetRuleParser.AtomContext,0)


        def Int32Type(self):
            return self.getToken(JetRuleParser.Int32Type, 0)

        def intExpr(self):
            return self.getTypedRuleContext(JetRuleParser.IntExprContext,0)


        def UInt32Type(self):
            return self.getToken(JetRuleParser.UInt32Type, 0)

        def uintExpr(self):
            return self.getTypedRuleContext(JetRuleParser.UintExprContext,0)


        def Int64Type(self):
            return self.getToken(JetRuleParser.Int64Type, 0)

        def UInt64Type(self):
            return self.getToken(JetRuleParser.UInt64Type, 0)

        def DoubleType(self):
            return self.getToken(JetRuleParser.DoubleType, 0)

        def doubleExpr(self):
            return self.getTypedRuleContext(JetRuleParser.DoubleExprContext,0)


        def StringType(self):
            return self.getToken(JetRuleParser.StringType, 0)

        def STRING(self):
            return self.getToken(JetRuleParser.STRING, 0)

        def DateType(self):
            return self.getToken(JetRuleParser.DateType, 0)

        def DatetimeType(self):
            return self.getToken(JetRuleParser.DatetimeType, 0)

        def BoolType(self):
            return self.getToken(JetRuleParser.BoolType, 0)

        def keywords(self):
            return self.getTypedRuleContext(JetRuleParser.KeywordsContext,0)


        def getRuleIndex(self):
            return JetRuleParser.RULE_objectAtom

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterObjectAtom" ):
                listener.enterObjectAtom(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitObjectAtom" ):
                listener.exitObjectAtom(self)




    def objectAtom(self):

        localctx = JetRuleParser.ObjectAtomContext(self, self._ctx, self.state)
        self.enterRule(localctx, 86, self.RULE_objectAtom)
        try:
            self.state = 677
            self._errHandler.sync(self)
            token = self._input.LA(1)
            if token in [JetRuleParser.T__11, JetRuleParser.Identifier]:
                self.enterOuterAlt(localctx, 1)
                self.state = 632
                self.atom()
                pass
            elif token in [JetRuleParser.Int32Type]:
                self.enterOuterAlt(localctx, 2)
                self.state = 633
                self.match(JetRuleParser.Int32Type)
                self.state = 634
                self.match(JetRuleParser.T__9)
                self.state = 635
                self.intExpr()
                self.state = 636
                self.match(JetRuleParser.T__10)
                pass
            elif token in [JetRuleParser.UInt32Type]:
                self.enterOuterAlt(localctx, 3)
                self.state = 638
                self.match(JetRuleParser.UInt32Type)
                self.state = 639
                self.match(JetRuleParser.T__9)
                self.state = 640
                self.uintExpr()
                self.state = 641
                self.match(JetRuleParser.T__10)
                pass
            elif token in [JetRuleParser.Int64Type]:
                self.enterOuterAlt(localctx, 4)
                self.state = 643
                self.match(JetRuleParser.Int64Type)
                self.state = 644
                self.match(JetRuleParser.T__9)
                self.state = 645
                self.intExpr()
                self.state = 646
                self.match(JetRuleParser.T__10)
                pass
            elif token in [JetRuleParser.UInt64Type]:
                self.enterOuterAlt(localctx, 5)
                self.state = 648
                self.match(JetRuleParser.UInt64Type)
                self.state = 649
                self.match(JetRuleParser.T__9)
                self.state = 650
                self.uintExpr()
                self.state = 651
                self.match(JetRuleParser.T__10)
                pass
            elif token in [JetRuleParser.DoubleType]:
                self.enterOuterAlt(localctx, 6)
                self.state = 653
                self.match(JetRuleParser.DoubleType)
                self.state = 654
                self.match(JetRuleParser.T__9)
                self.state = 655
                self.doubleExpr()
                self.state = 656
                self.match(JetRuleParser.T__10)
                pass
            elif token in [JetRuleParser.StringType]:
                self.enterOuterAlt(localctx, 7)
                self.state = 658
                self.match(JetRuleParser.StringType)
                self.state = 659
                self.match(JetRuleParser.T__9)
                self.state = 660
                self.match(JetRuleParser.STRING)
                self.state = 661
                self.match(JetRuleParser.T__10)
                pass
            elif token in [JetRuleParser.DateType]:
                self.enterOuterAlt(localctx, 8)
                self.state = 662
                self.match(JetRuleParser.DateType)
                self.state = 663
                self.match(JetRuleParser.T__9)
                self.state = 664
                self.match(JetRuleParser.STRING)
                self.state = 665
                self.match(JetRuleParser.T__10)
                pass
            elif token in [JetRuleParser.DatetimeType]:
                self.enterOuterAlt(localctx, 9)
                self.state = 666
                self.match(JetRuleParser.DatetimeType)
                self.state = 667
                self.match(JetRuleParser.T__9)
                self.state = 668
                self.match(JetRuleParser.STRING)
                self.state = 669
                self.match(JetRuleParser.T__10)
                pass
            elif token in [JetRuleParser.BoolType]:
                self.enterOuterAlt(localctx, 10)
                self.state = 670
                self.match(JetRuleParser.BoolType)
                self.state = 671
                self.match(JetRuleParser.T__9)
                self.state = 672
                self.match(JetRuleParser.STRING)
                self.state = 673
                self.match(JetRuleParser.T__10)
                pass
            elif token in [JetRuleParser.STRING]:
                self.enterOuterAlt(localctx, 11)
                self.state = 674
                self.match(JetRuleParser.STRING)
                pass
            elif token in [JetRuleParser.TRUE, JetRuleParser.FALSE, JetRuleParser.NULL]:
                self.enterOuterAlt(localctx, 12)
                self.state = 675
                localctx.kws = self.keywords()
                pass
            elif token in [JetRuleParser.PLUS, JetRuleParser.MINUS, JetRuleParser.DIGITS]:
                self.enterOuterAlt(localctx, 13)
                self.state = 676
                self.doubleExpr()
                pass
            else:
                raise NoViableAltException(self)

        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class KeywordsContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser

        def TRUE(self):
            return self.getToken(JetRuleParser.TRUE, 0)

        def FALSE(self):
            return self.getToken(JetRuleParser.FALSE, 0)

        def NULL(self):
            return self.getToken(JetRuleParser.NULL, 0)

        def getRuleIndex(self):
            return JetRuleParser.RULE_keywords

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterKeywords" ):
                listener.enterKeywords(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitKeywords" ):
                listener.exitKeywords(self)




    def keywords(self):

        localctx = JetRuleParser.KeywordsContext(self, self._ctx, self.state)
        self.enterRule(localctx, 88, self.RULE_keywords)
        self._la = 0 # Token type
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 679
            _la = self._input.LA(1)
            if not((((_la) & ~0x3f) == 0 and ((1 << _la) & ((1 << JetRuleParser.TRUE) | (1 << JetRuleParser.FALSE) | (1 << JetRuleParser.NULL))) != 0)):
                self._errHandler.recoverInline(self)
            else:
                self._errHandler.reportMatch(self)
                self.consume()
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class ExprTermContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser


        def getRuleIndex(self):
            return JetRuleParser.RULE_exprTerm

     
        def copyFrom(self, ctx:ParserRuleContext):
            super().copyFrom(ctx)


    class SelfExprTermContext(ExprTermContext):

        def __init__(self, parser, ctx:ParserRuleContext): # actually a JetRuleParser.ExprTermContext
            super().__init__(parser)
            self.selfExpr = None # ExprTermContext
            self.copyFrom(ctx)

        def exprTerm(self):
            return self.getTypedRuleContext(JetRuleParser.ExprTermContext,0)


        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterSelfExprTerm" ):
                listener.enterSelfExprTerm(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitSelfExprTerm" ):
                listener.exitSelfExprTerm(self)


    class BinaryExprTerm2Context(ExprTermContext):

        def __init__(self, parser, ctx:ParserRuleContext): # actually a JetRuleParser.ExprTermContext
            super().__init__(parser)
            self.lhs = None # ExprTermContext
            self.op = None # BinaryOpContext
            self.rhs = None # ExprTermContext
            self.copyFrom(ctx)

        def exprTerm(self, i:int=None):
            if i is None:
                return self.getTypedRuleContexts(JetRuleParser.ExprTermContext)
            else:
                return self.getTypedRuleContext(JetRuleParser.ExprTermContext,i)

        def binaryOp(self):
            return self.getTypedRuleContext(JetRuleParser.BinaryOpContext,0)


        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterBinaryExprTerm2" ):
                listener.enterBinaryExprTerm2(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitBinaryExprTerm2" ):
                listener.exitBinaryExprTerm2(self)


    class UnaryExprTermContext(ExprTermContext):

        def __init__(self, parser, ctx:ParserRuleContext): # actually a JetRuleParser.ExprTermContext
            super().__init__(parser)
            self.op = None # UnaryOpContext
            self.arg = None # ExprTermContext
            self.copyFrom(ctx)

        def unaryOp(self):
            return self.getTypedRuleContext(JetRuleParser.UnaryOpContext,0)

        def exprTerm(self):
            return self.getTypedRuleContext(JetRuleParser.ExprTermContext,0)


        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterUnaryExprTerm" ):
                listener.enterUnaryExprTerm(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitUnaryExprTerm" ):
                listener.exitUnaryExprTerm(self)


    class ObjectAtomExprTermContext(ExprTermContext):

        def __init__(self, parser, ctx:ParserRuleContext): # actually a JetRuleParser.ExprTermContext
            super().__init__(parser)
            self.ident = None # ObjectAtomContext
            self.copyFrom(ctx)

        def objectAtom(self):
            return self.getTypedRuleContext(JetRuleParser.ObjectAtomContext,0)


        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterObjectAtomExprTerm" ):
                listener.enterObjectAtomExprTerm(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitObjectAtomExprTerm" ):
                listener.exitObjectAtomExprTerm(self)


    class UnaryExprTerm3Context(ExprTermContext):

        def __init__(self, parser, ctx:ParserRuleContext): # actually a JetRuleParser.ExprTermContext
            super().__init__(parser)
            self.op = None # UnaryOpContext
            self.arg = None # ExprTermContext
            self.copyFrom(ctx)

        def unaryOp(self):
            return self.getTypedRuleContext(JetRuleParser.UnaryOpContext,0)

        def exprTerm(self):
            return self.getTypedRuleContext(JetRuleParser.ExprTermContext,0)


        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterUnaryExprTerm3" ):
                listener.enterUnaryExprTerm3(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitUnaryExprTerm3" ):
                listener.exitUnaryExprTerm3(self)


    class UnaryExprTerm2Context(ExprTermContext):

        def __init__(self, parser, ctx:ParserRuleContext): # actually a JetRuleParser.ExprTermContext
            super().__init__(parser)
            self.op = None # UnaryOpContext
            self.arg = None # ExprTermContext
            self.copyFrom(ctx)

        def unaryOp(self):
            return self.getTypedRuleContext(JetRuleParser.UnaryOpContext,0)

        def exprTerm(self):
            return self.getTypedRuleContext(JetRuleParser.ExprTermContext,0)


        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterUnaryExprTerm2" ):
                listener.enterUnaryExprTerm2(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitUnaryExprTerm2" ):
                listener.exitUnaryExprTerm2(self)


    class BinaryExprTermContext(ExprTermContext):

        def __init__(self, parser, ctx:ParserRuleContext): # actually a JetRuleParser.ExprTermContext
            super().__init__(parser)
            self.lhs = None # ExprTermContext
            self.op = None # BinaryOpContext
            self.rhs = None # ExprTermContext
            self.copyFrom(ctx)

        def exprTerm(self, i:int=None):
            if i is None:
                return self.getTypedRuleContexts(JetRuleParser.ExprTermContext)
            else:
                return self.getTypedRuleContext(JetRuleParser.ExprTermContext,i)

        def binaryOp(self):
            return self.getTypedRuleContext(JetRuleParser.BinaryOpContext,0)


        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterBinaryExprTerm" ):
                listener.enterBinaryExprTerm(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitBinaryExprTerm" ):
                listener.exitBinaryExprTerm(self)



    def exprTerm(self, _p:int=0):
        _parentctx = self._ctx
        _parentState = self.state
        localctx = JetRuleParser.ExprTermContext(self, self._ctx, _parentState)
        _prevctx = localctx
        _startState = 90
        self.enterRecursionRule(localctx, 90, self.RULE_exprTerm, _p)
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 706
            self._errHandler.sync(self)
            la_ = self._interp.adaptivePredict(self._input,64,self._ctx)
            if la_ == 1:
                localctx = JetRuleParser.BinaryExprTerm2Context(self, localctx)
                self._ctx = localctx
                _prevctx = localctx

                self.state = 682
                self.match(JetRuleParser.T__9)
                self.state = 683
                localctx.lhs = self.exprTerm(0)
                self.state = 684
                localctx.op = self.binaryOp()
                self.state = 685
                localctx.rhs = self.exprTerm(0)
                self.state = 686
                self.match(JetRuleParser.T__10)
                pass

            elif la_ == 2:
                localctx = JetRuleParser.UnaryExprTermContext(self, localctx)
                self._ctx = localctx
                _prevctx = localctx
                self.state = 688
                localctx.op = self.unaryOp()
                self.state = 689
                self.match(JetRuleParser.T__9)
                self.state = 690
                localctx.arg = self.exprTerm(0)
                self.state = 691
                self.match(JetRuleParser.T__10)
                pass

            elif la_ == 3:
                localctx = JetRuleParser.UnaryExprTerm2Context(self, localctx)
                self._ctx = localctx
                _prevctx = localctx
                self.state = 693
                self.match(JetRuleParser.T__9)
                self.state = 694
                localctx.op = self.unaryOp()
                self.state = 695
                localctx.arg = self.exprTerm(0)
                self.state = 696
                self.match(JetRuleParser.T__10)
                pass

            elif la_ == 4:
                localctx = JetRuleParser.SelfExprTermContext(self, localctx)
                self._ctx = localctx
                _prevctx = localctx
                self.state = 698
                self.match(JetRuleParser.T__9)
                self.state = 699
                localctx.selfExpr = self.exprTerm(0)
                self.state = 700
                self.match(JetRuleParser.T__10)
                pass

            elif la_ == 5:
                localctx = JetRuleParser.UnaryExprTerm3Context(self, localctx)
                self._ctx = localctx
                _prevctx = localctx
                self.state = 702
                localctx.op = self.unaryOp()
                self.state = 703
                localctx.arg = self.exprTerm(2)
                pass

            elif la_ == 6:
                localctx = JetRuleParser.ObjectAtomExprTermContext(self, localctx)
                self._ctx = localctx
                _prevctx = localctx
                self.state = 705
                localctx.ident = self.objectAtom()
                pass


            self._ctx.stop = self._input.LT(-1)
            self.state = 714
            self._errHandler.sync(self)
            _alt = self._interp.adaptivePredict(self._input,65,self._ctx)
            while _alt!=2 and _alt!=ATN.INVALID_ALT_NUMBER:
                if _alt==1:
                    if self._parseListeners is not None:
                        self.triggerExitRuleEvent()
                    _prevctx = localctx
                    localctx = JetRuleParser.BinaryExprTermContext(self, JetRuleParser.ExprTermContext(self, _parentctx, _parentState))
                    localctx.lhs = _prevctx
                    self.pushNewRecursionContext(localctx, _startState, self.RULE_exprTerm)
                    self.state = 708
                    if not self.precpred(self._ctx, 7):
                        from antlr4.error.Errors import FailedPredicateException
                        raise FailedPredicateException(self, "self.precpred(self._ctx, 7)")
                    self.state = 709
                    localctx.op = self.binaryOp()
                    self.state = 710
                    localctx.rhs = self.exprTerm(8) 
                self.state = 716
                self._errHandler.sync(self)
                _alt = self._interp.adaptivePredict(self._input,65,self._ctx)

        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.unrollRecursionContexts(_parentctx)
        return localctx


    class BinaryOpContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser

        def PLUS(self):
            return self.getToken(JetRuleParser.PLUS, 0)

        def EQ(self):
            return self.getToken(JetRuleParser.EQ, 0)

        def LT(self):
            return self.getToken(JetRuleParser.LT, 0)

        def LE(self):
            return self.getToken(JetRuleParser.LE, 0)

        def GT(self):
            return self.getToken(JetRuleParser.GT, 0)

        def GE(self):
            return self.getToken(JetRuleParser.GE, 0)

        def NE(self):
            return self.getToken(JetRuleParser.NE, 0)

        def REGEX2(self):
            return self.getToken(JetRuleParser.REGEX2, 0)

        def MINUS(self):
            return self.getToken(JetRuleParser.MINUS, 0)

        def MUL(self):
            return self.getToken(JetRuleParser.MUL, 0)

        def DIV(self):
            return self.getToken(JetRuleParser.DIV, 0)

        def OR(self):
            return self.getToken(JetRuleParser.OR, 0)

        def AND(self):
            return self.getToken(JetRuleParser.AND, 0)

        def Identifier(self):
            return self.getToken(JetRuleParser.Identifier, 0)

        def getRuleIndex(self):
            return JetRuleParser.RULE_binaryOp

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterBinaryOp" ):
                listener.enterBinaryOp(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitBinaryOp" ):
                listener.exitBinaryOp(self)




    def binaryOp(self):

        localctx = JetRuleParser.BinaryOpContext(self, self._ctx, self.state)
        self.enterRule(localctx, 92, self.RULE_binaryOp)
        self._la = 0 # Token type
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 717
            _la = self._input.LA(1)
            if not((((_la) & ~0x3f) == 0 and ((1 << _la) & ((1 << JetRuleParser.EQ) | (1 << JetRuleParser.LT) | (1 << JetRuleParser.LE) | (1 << JetRuleParser.GT) | (1 << JetRuleParser.GE) | (1 << JetRuleParser.NE) | (1 << JetRuleParser.REGEX2) | (1 << JetRuleParser.PLUS) | (1 << JetRuleParser.MINUS) | (1 << JetRuleParser.MUL) | (1 << JetRuleParser.DIV) | (1 << JetRuleParser.OR) | (1 << JetRuleParser.AND) | (1 << JetRuleParser.Identifier))) != 0)):
                self._errHandler.recoverInline(self)
            else:
                self._errHandler.reportMatch(self)
                self.consume()
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class UnaryOpContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser

        def NOT(self):
            return self.getToken(JetRuleParser.NOT, 0)

        def TOTEXT(self):
            return self.getToken(JetRuleParser.TOTEXT, 0)

        def Identifier(self):
            return self.getToken(JetRuleParser.Identifier, 0)

        def getRuleIndex(self):
            return JetRuleParser.RULE_unaryOp

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterUnaryOp" ):
                listener.enterUnaryOp(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitUnaryOp" ):
                listener.exitUnaryOp(self)




    def unaryOp(self):

        localctx = JetRuleParser.UnaryOpContext(self, self._ctx, self.state)
        self.enterRule(localctx, 94, self.RULE_unaryOp)
        self._la = 0 # Token type
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 719
            _la = self._input.LA(1)
            if not((((_la) & ~0x3f) == 0 and ((1 << _la) & ((1 << JetRuleParser.NOT) | (1 << JetRuleParser.TOTEXT) | (1 << JetRuleParser.Identifier))) != 0)):
                self._errHandler.recoverInline(self)
            else:
                self._errHandler.reportMatch(self)
                self.consume()
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx


    class TripleStmtContext(ParserRuleContext):
        __slots__ = 'parser'

        def __init__(self, parser, parent:ParserRuleContext=None, invokingState:int=-1):
            super().__init__(parent, invokingState)
            self.parser = parser
            self.s = None # AtomContext
            self.p = None # AtomContext
            self.o = None # ObjectAtomContext

        def TRIPLE(self):
            return self.getToken(JetRuleParser.TRIPLE, 0)

        def SEMICOLON(self):
            return self.getToken(JetRuleParser.SEMICOLON, 0)

        def atom(self, i:int=None):
            if i is None:
                return self.getTypedRuleContexts(JetRuleParser.AtomContext)
            else:
                return self.getTypedRuleContext(JetRuleParser.AtomContext,i)


        def objectAtom(self):
            return self.getTypedRuleContext(JetRuleParser.ObjectAtomContext,0)


        def getRuleIndex(self):
            return JetRuleParser.RULE_tripleStmt

        def enterRule(self, listener:ParseTreeListener):
            if hasattr( listener, "enterTripleStmt" ):
                listener.enterTripleStmt(self)

        def exitRule(self, listener:ParseTreeListener):
            if hasattr( listener, "exitTripleStmt" ):
                listener.exitTripleStmt(self)




    def tripleStmt(self):

        localctx = JetRuleParser.TripleStmtContext(self, self._ctx, self.state)
        self.enterRule(localctx, 96, self.RULE_tripleStmt)
        try:
            self.enterOuterAlt(localctx, 1)
            self.state = 721
            self.match(JetRuleParser.TRIPLE)
            self.state = 722
            self.match(JetRuleParser.T__9)
            self.state = 723
            localctx.s = self.atom()
            self.state = 724
            self.match(JetRuleParser.T__2)
            self.state = 725
            localctx.p = self.atom()
            self.state = 726
            self.match(JetRuleParser.T__2)
            self.state = 727
            localctx.o = self.objectAtom()
            self.state = 728
            self.match(JetRuleParser.T__10)
            self.state = 729
            self.match(JetRuleParser.SEMICOLON)
        except RecognitionException as re:
            localctx.exception = re
            self._errHandler.reportError(self, re)
            self._errHandler.recover(self, re)
        finally:
            self.exitRule()
        return localctx



    def sempred(self, localctx:RuleContext, ruleIndex:int, predIndex:int):
        if self._predicates == None:
            self._predicates = dict()
        self._predicates[45] = self.exprTerm_sempred
        pred = self._predicates.get(ruleIndex, None)
        if pred is None:
            raise Exception("No predicate with index:" + str(ruleIndex))
        else:
            return pred(localctx, predIndex)

    def exprTerm_sempred(self, localctx:ExprTermContext, predIndex:int):
            if predIndex == 0:
                return self.precpred(self._ctx, 7)
         




