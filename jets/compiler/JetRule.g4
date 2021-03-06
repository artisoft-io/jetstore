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
defineJetStoreConfigStmt: jetstoreConfig '{'
    COMMENT*
    jetstoreConfigSeq
    COMMENT*
  '}' SEMICOLON;

jetstoreConfig: JETSCONFIG | MAIN;
jetstoreConfigSeq: jetstoreConfigItem (',' COMMENT* jetstoreConfigItem)* ;
jetstoreConfigItem
  : configKey=MaxLooping ASSIGN configValue=uintExpr
  | configKey=MaxRuleExec ASSIGN configValue=uintExpr
  | configKey=InputType ASSIGN '[' COMMENT* rdfTypeList+=declIdentifier (','COMMENT* rdfTypeList+=declIdentifier)* COMMENT* ']'
  ;

// --------------------------------------------------------------------------------------
// Define Class Statements
// --------------------------------------------------------------------------------------
defineClassStmt: CLASS className=declIdentifier '{' 
    COMMENT* 
    classStmt (',' COMMENT* classStmt)*
    COMMENT*
  '}' SEMICOLON;

classStmt
  : BaseClasses ASSIGN '[' COMMENT* subClassOfStmt (',' COMMENT* subClassOfStmt)* COMMENT* ']'
  | DataProperties ASSIGN '[' COMMENT* dataPropertyDefinitions (','COMMENT* dataPropertyDefinitions)* COMMENT* ']'
  | GroupingProperties ASSIGN '[' COMMENT* groupingPropertyStmt (',' COMMENT* groupingPropertyStmt)* COMMENT* ']'
  | asTableStmt
  ;

subClassOfStmt: baseClassName=declIdentifier;
dataPropertyDefinitions: dataPName=declIdentifier 'as' array=ARRAY? dataPType=dataPropertyType;
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
  | ResourceType
;
groupingPropertyStmt: groupingPropertyName=declIdentifier;
asTableStmt: AsTable ASSIGN asTable=asTableFlag;
asTableFlag: TRUE | FALSE;

// --------------------------------------------------------------------------------------
// Define Rule Sequence Statements
// --------------------------------------------------------------------------------------
defineRuleSeqStmt: RULESEQ ruleseqName=Identifier '{'
    COMMENT*
    MainRuleSets ASSIGN '[' COMMENT* ruleSetSeq COMMENT* ']' ','?
    COMMENT*
  '}' SEMICOLON;

ruleSetSeq: ruleSetDefinitions (',' COMMENT* ruleSetDefinitions)* ;
ruleSetDefinitions: rsName=STRING;

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
    Columns ASSIGN '[' COMMENT* columnDefSeq COMMENT* ']' ','?
    COMMENT*
  '}' SEMICOLON;

csvLocation
  : TableName ASSIGN tblStorageName=declIdentifier ',' 
  | CSVFileName ASSIGN csvFileName=STRING ','
  ;

stringList: '[' seqCtx=stringSeq? ']';
stringSeq: slist+=STRING (',' slist+=STRING)* ;

columnDefSeq: columnDefinitions (',' COMMENT* columnDefinitions)* ;
columnDefinitions: 
  columnName=STRING 'as' array=ARRAY? columnType=dataPropertyType;

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
GroupingProperties: '$grouping_properties';

// JetStore Config
MAIN: 'main';
JETSCONFIG: 'jetstore_config';
MaxLooping: '$max_looping';        // Rule looping, default is 0, no looping
MaxRuleExec: '$max_rule_exec';     // Max number of times a rule can fire, default 10,000
InputType: '$input_types';         // RDF type of input record to rule set. rdf:Thing means any rdf type.

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