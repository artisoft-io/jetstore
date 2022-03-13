/**
 * JetRule grammar
 */
grammar JetRule;

// The main entry point for parsing a JetRule file.
jetrule: statement* EOF;

statement
  : jetCompilerDirectiveStmt
  | defineLiteralStmt  
  | defineResourceStmt 
  | lookupTableStmt
  | jetRuleStmt
  | COMMENT            
  ;

// --------------------------------------------------------------------------------------
// Define Jet Compiler Directive Statements
// --------------------------------------------------------------------------------------
jetCompilerDirectiveStmt:  
  JetCompilerDirective  
  varName=declIdentifier 
  ASSIGN declValue=STRING 
  SEMICOLON
;

// --------------------------------------------------------------------------------------
// Define Literal Statements
// --------------------------------------------------------------------------------------
defineLiteralStmt
  : int32LiteralStmt    
  | uInt32LiteralStmt   
  | int64LiteralStmt    
  | uInt64LiteralStmt   
  | doubleLiteralStmt   
  | stringLiteralStmt
  | dateLiteralStmt
  | datetimeLiteralStmt
  ;

int32LiteralStmt:    varType=Int32Type    varName=declIdentifier ASSIGN declValue=intExpr    SEMICOLON;
uInt32LiteralStmt:   varType=UInt32Type   varName=declIdentifier ASSIGN declValue=uintExpr   SEMICOLON;
int64LiteralStmt:    varType=Int64Type    varName=declIdentifier ASSIGN declValue=intExpr    SEMICOLON;
uInt64LiteralStmt:   varType=UInt64Type   varName=declIdentifier ASSIGN declValue=uintExpr   SEMICOLON;
doubleLiteralStmt:   varType=DoubleType   varName=declIdentifier ASSIGN declValue=doubleExpr SEMICOLON;
stringLiteralStmt:   varType=StringType   varName=declIdentifier ASSIGN declValue=STRING     SEMICOLON;
dateLiteralStmt:     varType=DateType     varName=declIdentifier ASSIGN declValue=STRING     SEMICOLON;
datetimeLiteralStmt: varType=DatetimeType varName=declIdentifier ASSIGN declValue=STRING     SEMICOLON;

intExpr
  : '+' intExpr  
  | '-' intExpr  
  | DIGITS
  ;

uintExpr
  : '+' uintExpr
  | DIGITS
  ;

doubleExpr
  : '+' doubleExpr
  | '-' doubleExpr
  | DIGITS ('.' DIGITS)?
  ;

declIdentifier
  : Identifier ':' Identifier
  | Identifier ':' STRING
  | Identifier
  ;

// --------------------------------------------------------------------------------------
// Define Resource Statements
// --------------------------------------------------------------------------------------
defineResourceStmt
  : namedResourceStmt
  | volatileResourceStmt
  ;

namedResourceStmt:    ResourceType         resName=declIdentifier ASSIGN resCtx=resourceValue SEMICOLON;
volatileResourceStmt: resType=VolatileResourceType resName=declIdentifier ASSIGN resVal=STRING SEMICOLON;

resourceValue
  : kws=keywords
  | resVal=CreateUUIDResource
  | resVal=STRING
  ;

// --------------------------------------------------------------------------------------
// Define Lookup Table
// --------------------------------------------------------------------------------------
lookupTableStmt: LookupTable lookupName=declIdentifier '{' 
    COMMENT*
    TableName ASSIGN tblStorageName=Identifier ',' 
    COMMENT*
    Key ASSIGN tblKeys=stringList ',' 
    COMMENT*
    Columns ASSIGN tblColumns=stringList 
    COMMENT*
  '}' SEMICOLON;

stringList: '[' seqCtx=stringSeq? ']';
stringSeq: slist+=STRING (',' slist+=STRING)* ;

// --------------------------------------------------------------------------------------
// Define Jet Rule
// --------------------------------------------------------------------------------------
jetRuleStmt: '[' ruleName=Identifier ruleProperties* ']' ':' 
    COMMENT*
    (antecedent COMMENT*)+ 
    '->' 
    COMMENT*
    (consequent COMMENT*)+
  SEMICOLON ;

ruleProperties: ',' key=Identifier ASSIGN valCtx=propertyValue ;
propertyValue: ( val=STRING | val=TRUE | val=FALSE | intval=intExpr ) ;

antecedent: n=NOT? '(' s=atom p=atom o=objectAtom ')' '.'? ( '[' f=exprTerm ']' '.'? )? ;
consequent: '(' s=atom p=atom o=exprTerm ')' '.'? ;

atom
  : '?' Identifier
  | declIdentifier
  ;

objectAtom
  : atom                         
  | Int32Type '(' intExpr ')'    
  | UInt32Type '(' uintExpr ')'  
  | Int64Type '(' intExpr ')'    
  | UInt64Type '(' uintExpr ')'  
  | DoubleType '(' doubleExpr ')'
  | StringType '(' STRING ')'
  | DateType '(' STRING ')'
  | DatetimeType '(' STRING ')'
  | STRING
  | kws=keywords
  | doubleExpr
  ;

keywords
  : TRUE  
  | FALSE 
  | NULL 
  ;

exprTerm
  : lhs=exprTerm op=binaryOp rhs=exprTerm          # BinaryExprTerm
  | '(' lhs=exprTerm op=binaryOp rhs=exprTerm ')'  # BinaryExprTerm2
  | op=unaryOp '(' arg=exprTerm ')'                # UnaryExprTerm
  | '(' op=unaryOp arg=exprTerm ')'                # UnaryExprTerm2
  | '(' selfExpr=exprTerm ')'                      # SelfExprTerm
  | op=unaryOp arg=exprTerm                        # UnaryExprTerm3
  | ident=objectAtom                               # ObjectAtomExprTerm
  ;

binaryOp
  : PLUS
  | EQ
  | LT
  | LE
  | GT
  | GE
  | NE
  | REGEX2
  | MINUS
  | MUL
  | DIV
  | OR
  | AND
  | Identifier
  ;

unaryOp
  : NOT
  | TOTEXT
  | Identifier
  ;

// ======================================================================================
// Lexer section
// --------------------------------------------------------------------------------------
// Jet Compiler directives and decorators
JetCompilerDirective: '@JetCompilerDirective';

// Literals and resources
Int32Type: 'int';
UInt32Type: 'uint';
Int64Type: 'long';
UInt64Type: 'ulong';
DoubleType: 'double';
StringType: 'text';
DateType: 'date';
DatetimeType: 'datetime';

ResourceType: 'resource';
VolatileResourceType: 'volatile_resource';

// Functions
CreateUUIDResource: 'create_uuid_resource()';

// Properties for lookup tables
LookupTable: 'lookup_table';
TableName: '$table_name';
Key: '$key';
Columns: '$columns';

// Keywords / symbols
TRUE: 'true';
FALSE: 'false';
NULL: 'null';

// Unary operator
NOT: 'not';
TOTEXT: 'toText';

// Binary operator
EQ: '==';
LT: '<';
LE: '<=';
GT: '>';
GE: '>=';
NE: '!=';
REGEX2: 'r?';
PLUS: '+';
MINUS: '-';
MUL: '*';
DIV: '/';
OR: 'or';
AND: 'and';

SEMICOLON: ';';
ASSIGN: '=';

Identifier:	NONDIGIT ( NONDIGIT | DIGITS)*;
fragment NONDIGIT: [a-zA-Z_];
DIGITS: [0-9]+;

STRING: '"' (ESC|.)*? '"' ;
fragment ESC: '\\"'|'\\\\';
// STRING: '"' Schar* '"';
// fragment Schar: ~ ["\\\r\n] | '\\"' ;

COMMENT: '#' Cchar*;
fragment Cchar: ~ [\r\n];

WS : [ \t\r\n]+ -> skip ; // skip spaces, tabs, newlines