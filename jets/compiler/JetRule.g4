/**
 * JetRule grammar
 */
grammar JetRule;

// The main entry point for parsing a JetRule file.
jetrule: statement* EOF;

statement
  : jetCompilerDirectiveStmt
  | defineJetStoreConfigStmt  
  | defineLiteralStmt  
  | defineClassStmt  
  | defineRuleSeqStmt 
  | defineResourceStmt 
  | lookupTableStmt
  | jetRuleStmt
  | tripleStmt
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
// Define JetStore Config Statements
// --------------------------------------------------------------------------------------
defineJetStoreConfigStmt: JETSCONFIG '{'
    COMMENT*
    jetstoreConfigItem
    COMMENT*
  '}' SEMICOLON;

jetstoreConfigItem: rsName=STRING (',' COMMENT* ruleSetDefinitions)* ;

// --------------------------------------------------------------------------------------
// Define Class Statements
// --------------------------------------------------------------------------------------
defineClassStmt: CLASS className=declIdentifier '{' 
    COMMENT*
    BaseClasses ASSIGN '[' COMMENT* subClassOfStmt COMMENT* ']' ','
    COMMENT*
    DataProperties ASSIGN '[' COMMENT* dataPropertyDefinitions COMMENT* ']'
    (asTableStmt)?
    COMMENT*
  '}' SEMICOLON;

subClassOfStmt: baseClassName=declIdentifier (',' COMMENT* subClassOfStmt)* ;
dataPropertyDefinitions: dataPName=declIdentifier 'as' array=ARRAY? dataPType=dataPropertyType (',' COMMENT* dataPropertyDefinitions)* ;
asTableStmt: ',' COMMENT* AsTable ASSIGN asTable=asTableFlag;
asTableFlag: TRUE | FALSE;
dataPropertyType
  : Int32Type
  | UInt32Type
  | Int64Type
  | UInt64Type
  | DoubleType
  | StringType
  | DateType
  | DatetimeType
  | BoolType
;

// --------------------------------------------------------------------------------------
// Define Rule Sequence Statements
// --------------------------------------------------------------------------------------
defineRuleSeqStmt: RULESEQ ruleseqName=Identifier '{'
    COMMENT*
    MainRuleSets ASSIGN '[' COMMENT* ruleSetDefinitions COMMENT* ']' ','?
    COMMENT*
  '}' SEMICOLON;

ruleSetDefinitions: rsName=STRING (',' COMMENT* ruleSetDefinitions)* ;

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
  | booleanLiteralStmt
  ;

int32LiteralStmt:    varType=Int32Type    varName=declIdentifier ASSIGN declValue=intExpr    SEMICOLON;
uInt32LiteralStmt:   varType=UInt32Type   varName=declIdentifier ASSIGN declValue=uintExpr   SEMICOLON;
int64LiteralStmt:    varType=Int64Type    varName=declIdentifier ASSIGN declValue=intExpr    SEMICOLON;
uInt64LiteralStmt:   varType=UInt64Type   varName=declIdentifier ASSIGN declValue=uintExpr   SEMICOLON;
doubleLiteralStmt:   varType=DoubleType   varName=declIdentifier ASSIGN declValue=doubleExpr SEMICOLON;
stringLiteralStmt:   varType=StringType   varName=declIdentifier ASSIGN declValue=STRING     SEMICOLON;
dateLiteralStmt:     varType=DateType     varName=declIdentifier ASSIGN declValue=STRING     SEMICOLON;
datetimeLiteralStmt: varType=DatetimeType varName=declIdentifier ASSIGN declValue=STRING     SEMICOLON;
booleanLiteralStmt:  varType=BoolType     varName=declIdentifier ASSIGN declValue=STRING     SEMICOLON;

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
    csvLocation
    COMMENT*
    Key ASSIGN tblKeys=stringList ',' 
    COMMENT*
    Columns ASSIGN '[' COMMENT* columnDefinitions COMMENT* ']' ','?
    COMMENT*
  '}' SEMICOLON;

csvLocation
  : TableName ASSIGN tblStorageName=Identifier ',' 
  | CSVFileName ASSIGN csvFileName=STRING ','
  ;

stringList: '[' seqCtx=stringSeq? ']';
stringSeq: slist+=STRING (',' slist+=STRING)* ;

columnDefinitions: 
  columnName=STRING 'as' array=ARRAY? columnType=dataPropertyType (',' COMMENT* columnDefinitions)* ;

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
  | BoolType '(' STRING ')'
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


// --------------------------------------------------------------------------------------
// Define Triple
// --------------------------------------------------------------------------------------
tripleStmt: TRIPLE '(' s=atom ',' p=atom ',' o=objectAtom ')' SEMICOLON ; 

// ======================================================================================
// Lexer section
// --------------------------------------------------------------------------------------
// Jet Compiler directives and decorators
JetCompilerDirective: '@JetCompilerDirective';

// Class statement for model definition
CLASS: 'class';
BaseClasses: '$base_classes';
AsTable: '$as_table';
DataProperties: '$data_properties';
ARRAY: 'array of';

// JetStore Config
JETSCONFIG: 'jetstore_config';
MaxLooping: '$max_looping';        // Rule looping, default is 0, no looping
MaxRuleExec: '$max_rule_exec';     // Max number of times a rule can fire, default 10,000

// Rule Sequence
RULESEQ: 'rule_sequence';
MainRuleSets: '$main_rule_sets';

// Triple statement
TRIPLE: 'triple';

// Literals and resources
Int32Type: 'int';
UInt32Type: 'uint';
Int64Type: 'long';
UInt64Type: 'ulong';
DoubleType: 'double';
StringType: 'text';
DateType: 'date';
DatetimeType: 'datetime';
BoolType: 'bool';

ResourceType: 'resource';
VolatileResourceType: 'volatile_resource';

// Functions
CreateUUIDResource: 'create_uuid_resource()';

// Properties for lookup tables
LookupTable: 'lookup_table';
TableName: '$table_name';
CSVFileName: '$csv_file';
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