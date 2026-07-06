// Generated from /home/michel/projects/repos/jetstore/jets/compilerv2/compiler/JetRule.g4 by ANTLR 4.13.1
import org.antlr.v4.runtime.atn.*;
import org.antlr.v4.runtime.dfa.DFA;
import org.antlr.v4.runtime.*;
import org.antlr.v4.runtime.misc.*;
import org.antlr.v4.runtime.tree.*;
import java.util.List;
import java.util.Iterator;
import java.util.ArrayList;

@SuppressWarnings({"all", "warnings", "unchecked", "unused", "cast", "CheckReturnValue"})
public class JetRuleParser extends Parser {
	static { RuntimeMetaData.checkVersion("4.13.1", RuntimeMetaData.VERSION); }

	protected static final DFA[] _decisionToDFA;
	protected static final PredictionContextCache _sharedContextCache =
		new PredictionContextCache();
	public static final int
		T__0=1, T__1=2, T__2=3, T__3=4, T__4=5, T__5=6, T__6=7, T__7=8, T__8=9, 
		T__9=10, T__10=11, T__11=12, JetCompilerDirective=13, CLASS=14, BaseClasses=15, 
		AsTable=16, DataProperties=17, ObjectProperties=18, ARRAY=19, GroupingProperties=20, 
		MAIN=21, JETSCONFIG=22, MaxLooping=23, MaxRuleExec=24, InputType=25, RULESEQ=26, 
		MainRuleSets=27, TRIPLE=28, Int32Type=29, UInt32Type=30, Int64Type=31, 
		UInt64Type=32, DoubleType=33, StringType=34, DateType=35, DatetimeType=36, 
		BoolType=37, ResourceType=38, VolatileResourceType=39, CreateUUIDResource=40, 
		LookupTable=41, TableName=42, CSVFileName=43, Key=44, Columns=45, TRUE=46, 
		FALSE=47, NULL=48, NOT=49, TOTEXT=50, EQ=51, LT=52, LE=53, GT=54, GE=55, 
		NE=56, REGEX2=57, PLUS=58, MINUS=59, MUL=60, DIV=61, OR=62, AND=63, SEMICOLON=64, 
		ASSIGN=65, Identifier=66, DIGITS=67, STRING=68, COMMENT=69, WS=70;
	public static final int
		RULE_jetrule = 0, RULE_statement = 1, RULE_jetCompilerDirectiveStmt = 2, 
		RULE_defineJetStoreConfigStmt = 3, RULE_jetstoreConfig = 4, RULE_jetstoreConfigSeq = 5, 
		RULE_jetstoreConfigItem = 6, RULE_defineClassStmt = 7, RULE_classStmt = 8, 
		RULE_subClassOfStmt = 9, RULE_dataPropertyDefinitions = 10, RULE_objectPropertyDefinitions = 11, 
		RULE_dataPropertyType = 12, RULE_groupingPropertyStmt = 13, RULE_asTableStmt = 14, 
		RULE_asTableFlag = 15, RULE_defineRuleSeqStmt = 16, RULE_ruleSetSeq = 17, 
		RULE_ruleSetDefinitions = 18, RULE_defineLiteralStmt = 19, RULE_int32LiteralStmt = 20, 
		RULE_uInt32LiteralStmt = 21, RULE_int64LiteralStmt = 22, RULE_uInt64LiteralStmt = 23, 
		RULE_doubleLiteralStmt = 24, RULE_stringLiteralStmt = 25, RULE_dateLiteralStmt = 26, 
		RULE_datetimeLiteralStmt = 27, RULE_booleanLiteralStmt = 28, RULE_intExpr = 29, 
		RULE_uintExpr = 30, RULE_doubleExpr = 31, RULE_declIdentifier = 32, RULE_defineResourceStmt = 33, 
		RULE_namedResourceStmt = 34, RULE_volatileResourceStmt = 35, RULE_resourceValue = 36, 
		RULE_lookupTableStmt = 37, RULE_csvLocation = 38, RULE_stringList = 39, 
		RULE_stringSeq = 40, RULE_columnDefSeq = 41, RULE_columnDefinitions = 42, 
		RULE_jetRuleStmt = 43, RULE_ruleProperties = 44, RULE_propertyValue = 45, 
		RULE_antecedent = 46, RULE_consequent = 47, RULE_atom = 48, RULE_objectAtom = 49, 
		RULE_keywords = 50, RULE_exprTerm = 51, RULE_binaryOp = 52, RULE_unaryOp = 53, 
		RULE_tripleStmt = 54;
	private static String[] makeRuleNames() {
		return new String[] {
			"jetrule", "statement", "jetCompilerDirectiveStmt", "defineJetStoreConfigStmt", 
			"jetstoreConfig", "jetstoreConfigSeq", "jetstoreConfigItem", "defineClassStmt", 
			"classStmt", "subClassOfStmt", "dataPropertyDefinitions", "objectPropertyDefinitions", 
			"dataPropertyType", "groupingPropertyStmt", "asTableStmt", "asTableFlag", 
			"defineRuleSeqStmt", "ruleSetSeq", "ruleSetDefinitions", "defineLiteralStmt", 
			"int32LiteralStmt", "uInt32LiteralStmt", "int64LiteralStmt", "uInt64LiteralStmt", 
			"doubleLiteralStmt", "stringLiteralStmt", "dateLiteralStmt", "datetimeLiteralStmt", 
			"booleanLiteralStmt", "intExpr", "uintExpr", "doubleExpr", "declIdentifier", 
			"defineResourceStmt", "namedResourceStmt", "volatileResourceStmt", "resourceValue", 
			"lookupTableStmt", "csvLocation", "stringList", "stringSeq", "columnDefSeq", 
			"columnDefinitions", "jetRuleStmt", "ruleProperties", "propertyValue", 
			"antecedent", "consequent", "atom", "objectAtom", "keywords", "exprTerm", 
			"binaryOp", "unaryOp", "tripleStmt"
		};
	}
	public static final String[] ruleNames = makeRuleNames();

	private static String[] makeLiteralNames() {
		return new String[] {
			null, "'{'", "'}'", "','", "'['", "']'", "'as'", "'.'", "':'", "'->'", 
			"'('", "')'", "'?'", "'@JetCompilerDirective'", "'class'", "'$base_classes'", 
			"'$as_table'", "'$data_properties'", "'$object_properties'", "'array of'", 
			"'$grouping_properties'", "'main'", "'jetstore_config'", "'$max_looping'", 
			"'$max_rule_exec'", "'$input_types'", "'rule_sequence'", "'$main_rule_sets'", 
			"'triple'", "'int'", "'uint'", "'long'", "'ulong'", "'double'", "'text'", 
			"'date'", "'datetime'", "'bool'", "'resource'", "'volatile_resource'", 
			"'create_uuid_resource()'", "'lookup_table'", "'$table_name'", "'$csv_file'", 
			"'$key'", "'$columns'", "'true'", "'false'", "'null'", "'not'", "'toText'", 
			"'=='", "'<'", "'<='", "'>'", "'>='", "'!='", "'r?'", "'+'", "'-'", "'*'", 
			"'/'", "'or'", "'and'", "';'", "'='"
		};
	}
	private static final String[] _LITERAL_NAMES = makeLiteralNames();
	private static String[] makeSymbolicNames() {
		return new String[] {
			null, null, null, null, null, null, null, null, null, null, null, null, 
			null, "JetCompilerDirective", "CLASS", "BaseClasses", "AsTable", "DataProperties", 
			"ObjectProperties", "ARRAY", "GroupingProperties", "MAIN", "JETSCONFIG", 
			"MaxLooping", "MaxRuleExec", "InputType", "RULESEQ", "MainRuleSets", 
			"TRIPLE", "Int32Type", "UInt32Type", "Int64Type", "UInt64Type", "DoubleType", 
			"StringType", "DateType", "DatetimeType", "BoolType", "ResourceType", 
			"VolatileResourceType", "CreateUUIDResource", "LookupTable", "TableName", 
			"CSVFileName", "Key", "Columns", "TRUE", "FALSE", "NULL", "NOT", "TOTEXT", 
			"EQ", "LT", "LE", "GT", "GE", "NE", "REGEX2", "PLUS", "MINUS", "MUL", 
			"DIV", "OR", "AND", "SEMICOLON", "ASSIGN", "Identifier", "DIGITS", "STRING", 
			"COMMENT", "WS"
		};
	}
	private static final String[] _SYMBOLIC_NAMES = makeSymbolicNames();
	public static final Vocabulary VOCABULARY = new VocabularyImpl(_LITERAL_NAMES, _SYMBOLIC_NAMES);

	/**
	 * @deprecated Use {@link #VOCABULARY} instead.
	 */
	@Deprecated
	public static final String[] tokenNames;
	static {
		tokenNames = new String[_SYMBOLIC_NAMES.length];
		for (int i = 0; i < tokenNames.length; i++) {
			tokenNames[i] = VOCABULARY.getLiteralName(i);
			if (tokenNames[i] == null) {
				tokenNames[i] = VOCABULARY.getSymbolicName(i);
			}

			if (tokenNames[i] == null) {
				tokenNames[i] = "<INVALID>";
			}
		}
	}

	@Override
	@Deprecated
	public String[] getTokenNames() {
		return tokenNames;
	}

	@Override

	public Vocabulary getVocabulary() {
		return VOCABULARY;
	}

	@Override
	public String getGrammarFileName() { return "JetRule.g4"; }

	@Override
	public String[] getRuleNames() { return ruleNames; }

	@Override
	public String getSerializedATN() { return _serializedATN; }

	@Override
	public ATN getATN() { return _ATN; }

	public JetRuleParser(TokenStream input) {
		super(input);
		_interp = new ParserATNSimulator(this,_ATN,_decisionToDFA,_sharedContextCache);
	}

	@SuppressWarnings("CheckReturnValue")
	public static class JetruleContext extends ParserRuleContext {
		public TerminalNode EOF() { return getToken(JetRuleParser.EOF, 0); }
		public List<StatementContext> statement() {
			return getRuleContexts(StatementContext.class);
		}
		public StatementContext statement(int i) {
			return getRuleContext(StatementContext.class,i);
		}
		public JetruleContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_jetrule; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterJetrule(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitJetrule(this);
		}
	}

	public final JetruleContext jetrule() throws RecognitionException {
		JetruleContext _localctx = new JetruleContext(_ctx, getState());
		enterRule(_localctx, 0, RULE_jetrule);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(113);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while ((((_la) & ~0x3f) == 0 && ((1L << _la) & 3298339872784L) != 0) || _la==COMMENT) {
				{
				{
				setState(110);
				statement();
				}
				}
				setState(115);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			setState(116);
			match(EOF);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class StatementContext extends ParserRuleContext {
		public JetCompilerDirectiveStmtContext jetCompilerDirectiveStmt() {
			return getRuleContext(JetCompilerDirectiveStmtContext.class,0);
		}
		public DefineJetStoreConfigStmtContext defineJetStoreConfigStmt() {
			return getRuleContext(DefineJetStoreConfigStmtContext.class,0);
		}
		public DefineLiteralStmtContext defineLiteralStmt() {
			return getRuleContext(DefineLiteralStmtContext.class,0);
		}
		public DefineClassStmtContext defineClassStmt() {
			return getRuleContext(DefineClassStmtContext.class,0);
		}
		public DefineRuleSeqStmtContext defineRuleSeqStmt() {
			return getRuleContext(DefineRuleSeqStmtContext.class,0);
		}
		public DefineResourceStmtContext defineResourceStmt() {
			return getRuleContext(DefineResourceStmtContext.class,0);
		}
		public LookupTableStmtContext lookupTableStmt() {
			return getRuleContext(LookupTableStmtContext.class,0);
		}
		public JetRuleStmtContext jetRuleStmt() {
			return getRuleContext(JetRuleStmtContext.class,0);
		}
		public TripleStmtContext tripleStmt() {
			return getRuleContext(TripleStmtContext.class,0);
		}
		public TerminalNode COMMENT() { return getToken(JetRuleParser.COMMENT, 0); }
		public StatementContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_statement; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterStatement(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitStatement(this);
		}
	}

	public final StatementContext statement() throws RecognitionException {
		StatementContext _localctx = new StatementContext(_ctx, getState());
		enterRule(_localctx, 2, RULE_statement);
		try {
			setState(128);
			_errHandler.sync(this);
			switch (_input.LA(1)) {
			case JetCompilerDirective:
				enterOuterAlt(_localctx, 1);
				{
				setState(118);
				jetCompilerDirectiveStmt();
				}
				break;
			case MAIN:
			case JETSCONFIG:
				enterOuterAlt(_localctx, 2);
				{
				setState(119);
				defineJetStoreConfigStmt();
				}
				break;
			case Int32Type:
			case UInt32Type:
			case Int64Type:
			case UInt64Type:
			case DoubleType:
			case StringType:
			case DateType:
			case DatetimeType:
			case BoolType:
				enterOuterAlt(_localctx, 3);
				{
				setState(120);
				defineLiteralStmt();
				}
				break;
			case CLASS:
				enterOuterAlt(_localctx, 4);
				{
				setState(121);
				defineClassStmt();
				}
				break;
			case RULESEQ:
				enterOuterAlt(_localctx, 5);
				{
				setState(122);
				defineRuleSeqStmt();
				}
				break;
			case ResourceType:
			case VolatileResourceType:
				enterOuterAlt(_localctx, 6);
				{
				setState(123);
				defineResourceStmt();
				}
				break;
			case LookupTable:
				enterOuterAlt(_localctx, 7);
				{
				setState(124);
				lookupTableStmt();
				}
				break;
			case T__3:
				enterOuterAlt(_localctx, 8);
				{
				setState(125);
				jetRuleStmt();
				}
				break;
			case TRIPLE:
				enterOuterAlt(_localctx, 9);
				{
				setState(126);
				tripleStmt();
				}
				break;
			case COMMENT:
				enterOuterAlt(_localctx, 10);
				{
				setState(127);
				match(COMMENT);
				}
				break;
			default:
				throw new NoViableAltException(this);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class JetCompilerDirectiveStmtContext extends ParserRuleContext {
		public DeclIdentifierContext varName;
		public Token declValue;
		public TerminalNode JetCompilerDirective() { return getToken(JetRuleParser.JetCompilerDirective, 0); }
		public TerminalNode ASSIGN() { return getToken(JetRuleParser.ASSIGN, 0); }
		public TerminalNode SEMICOLON() { return getToken(JetRuleParser.SEMICOLON, 0); }
		public DeclIdentifierContext declIdentifier() {
			return getRuleContext(DeclIdentifierContext.class,0);
		}
		public TerminalNode STRING() { return getToken(JetRuleParser.STRING, 0); }
		public JetCompilerDirectiveStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_jetCompilerDirectiveStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterJetCompilerDirectiveStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitJetCompilerDirectiveStmt(this);
		}
	}

	public final JetCompilerDirectiveStmtContext jetCompilerDirectiveStmt() throws RecognitionException {
		JetCompilerDirectiveStmtContext _localctx = new JetCompilerDirectiveStmtContext(_ctx, getState());
		enterRule(_localctx, 4, RULE_jetCompilerDirectiveStmt);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(130);
			match(JetCompilerDirective);
			setState(131);
			((JetCompilerDirectiveStmtContext)_localctx).varName = declIdentifier();
			setState(132);
			match(ASSIGN);
			setState(133);
			((JetCompilerDirectiveStmtContext)_localctx).declValue = match(STRING);
			setState(134);
			match(SEMICOLON);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class DefineJetStoreConfigStmtContext extends ParserRuleContext {
		public JetstoreConfigContext jetstoreConfig() {
			return getRuleContext(JetstoreConfigContext.class,0);
		}
		public JetstoreConfigSeqContext jetstoreConfigSeq() {
			return getRuleContext(JetstoreConfigSeqContext.class,0);
		}
		public TerminalNode SEMICOLON() { return getToken(JetRuleParser.SEMICOLON, 0); }
		public List<TerminalNode> COMMENT() { return getTokens(JetRuleParser.COMMENT); }
		public TerminalNode COMMENT(int i) {
			return getToken(JetRuleParser.COMMENT, i);
		}
		public DefineJetStoreConfigStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_defineJetStoreConfigStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterDefineJetStoreConfigStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitDefineJetStoreConfigStmt(this);
		}
	}

	public final DefineJetStoreConfigStmtContext defineJetStoreConfigStmt() throws RecognitionException {
		DefineJetStoreConfigStmtContext _localctx = new DefineJetStoreConfigStmtContext(_ctx, getState());
		enterRule(_localctx, 6, RULE_defineJetStoreConfigStmt);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(136);
			jetstoreConfig();
			setState(137);
			match(T__0);
			setState(141);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==COMMENT) {
				{
				{
				setState(138);
				match(COMMENT);
				}
				}
				setState(143);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			setState(144);
			jetstoreConfigSeq();
			setState(148);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==COMMENT) {
				{
				{
				setState(145);
				match(COMMENT);
				}
				}
				setState(150);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			setState(151);
			match(T__1);
			setState(152);
			match(SEMICOLON);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class JetstoreConfigContext extends ParserRuleContext {
		public TerminalNode JETSCONFIG() { return getToken(JetRuleParser.JETSCONFIG, 0); }
		public TerminalNode MAIN() { return getToken(JetRuleParser.MAIN, 0); }
		public JetstoreConfigContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_jetstoreConfig; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterJetstoreConfig(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitJetstoreConfig(this);
		}
	}

	public final JetstoreConfigContext jetstoreConfig() throws RecognitionException {
		JetstoreConfigContext _localctx = new JetstoreConfigContext(_ctx, getState());
		enterRule(_localctx, 8, RULE_jetstoreConfig);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(154);
			_la = _input.LA(1);
			if ( !(_la==MAIN || _la==JETSCONFIG) ) {
			_errHandler.recoverInline(this);
			}
			else {
				if ( _input.LA(1)==Token.EOF ) matchedEOF = true;
				_errHandler.reportMatch(this);
				consume();
			}
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class JetstoreConfigSeqContext extends ParserRuleContext {
		public List<JetstoreConfigItemContext> jetstoreConfigItem() {
			return getRuleContexts(JetstoreConfigItemContext.class);
		}
		public JetstoreConfigItemContext jetstoreConfigItem(int i) {
			return getRuleContext(JetstoreConfigItemContext.class,i);
		}
		public List<TerminalNode> COMMENT() { return getTokens(JetRuleParser.COMMENT); }
		public TerminalNode COMMENT(int i) {
			return getToken(JetRuleParser.COMMENT, i);
		}
		public JetstoreConfigSeqContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_jetstoreConfigSeq; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterJetstoreConfigSeq(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitJetstoreConfigSeq(this);
		}
	}

	public final JetstoreConfigSeqContext jetstoreConfigSeq() throws RecognitionException {
		JetstoreConfigSeqContext _localctx = new JetstoreConfigSeqContext(_ctx, getState());
		enterRule(_localctx, 10, RULE_jetstoreConfigSeq);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(156);
			jetstoreConfigItem();
			setState(167);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==T__2) {
				{
				{
				setState(157);
				match(T__2);
				setState(161);
				_errHandler.sync(this);
				_la = _input.LA(1);
				while (_la==COMMENT) {
					{
					{
					setState(158);
					match(COMMENT);
					}
					}
					setState(163);
					_errHandler.sync(this);
					_la = _input.LA(1);
				}
				setState(164);
				jetstoreConfigItem();
				}
				}
				setState(169);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class JetstoreConfigItemContext extends ParserRuleContext {
		public Token configKey;
		public UintExprContext configValue;
		public DeclIdentifierContext declIdentifier;
		public List<DeclIdentifierContext> rdfTypeList = new ArrayList<DeclIdentifierContext>();
		public TerminalNode ASSIGN() { return getToken(JetRuleParser.ASSIGN, 0); }
		public TerminalNode MaxLooping() { return getToken(JetRuleParser.MaxLooping, 0); }
		public UintExprContext uintExpr() {
			return getRuleContext(UintExprContext.class,0);
		}
		public TerminalNode MaxRuleExec() { return getToken(JetRuleParser.MaxRuleExec, 0); }
		public TerminalNode InputType() { return getToken(JetRuleParser.InputType, 0); }
		public List<DeclIdentifierContext> declIdentifier() {
			return getRuleContexts(DeclIdentifierContext.class);
		}
		public DeclIdentifierContext declIdentifier(int i) {
			return getRuleContext(DeclIdentifierContext.class,i);
		}
		public List<TerminalNode> COMMENT() { return getTokens(JetRuleParser.COMMENT); }
		public TerminalNode COMMENT(int i) {
			return getToken(JetRuleParser.COMMENT, i);
		}
		public JetstoreConfigItemContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_jetstoreConfigItem; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterJetstoreConfigItem(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitJetstoreConfigItem(this);
		}
	}

	public final JetstoreConfigItemContext jetstoreConfigItem() throws RecognitionException {
		JetstoreConfigItemContext _localctx = new JetstoreConfigItemContext(_ctx, getState());
		enterRule(_localctx, 12, RULE_jetstoreConfigItem);
		int _la;
		try {
			setState(207);
			_errHandler.sync(this);
			switch (_input.LA(1)) {
			case MaxLooping:
				enterOuterAlt(_localctx, 1);
				{
				setState(170);
				((JetstoreConfigItemContext)_localctx).configKey = match(MaxLooping);
				setState(171);
				match(ASSIGN);
				setState(172);
				((JetstoreConfigItemContext)_localctx).configValue = uintExpr();
				}
				break;
			case MaxRuleExec:
				enterOuterAlt(_localctx, 2);
				{
				setState(173);
				((JetstoreConfigItemContext)_localctx).configKey = match(MaxRuleExec);
				setState(174);
				match(ASSIGN);
				setState(175);
				((JetstoreConfigItemContext)_localctx).configValue = uintExpr();
				}
				break;
			case InputType:
				enterOuterAlt(_localctx, 3);
				{
				setState(176);
				((JetstoreConfigItemContext)_localctx).configKey = match(InputType);
				setState(177);
				match(ASSIGN);
				setState(178);
				match(T__3);
				setState(182);
				_errHandler.sync(this);
				_la = _input.LA(1);
				while (_la==COMMENT) {
					{
					{
					setState(179);
					match(COMMENT);
					}
					}
					setState(184);
					_errHandler.sync(this);
					_la = _input.LA(1);
				}
				setState(185);
				((JetstoreConfigItemContext)_localctx).declIdentifier = declIdentifier();
				((JetstoreConfigItemContext)_localctx).rdfTypeList.add(((JetstoreConfigItemContext)_localctx).declIdentifier);
				setState(196);
				_errHandler.sync(this);
				_la = _input.LA(1);
				while (_la==T__2) {
					{
					{
					setState(186);
					match(T__2);
					setState(190);
					_errHandler.sync(this);
					_la = _input.LA(1);
					while (_la==COMMENT) {
						{
						{
						setState(187);
						match(COMMENT);
						}
						}
						setState(192);
						_errHandler.sync(this);
						_la = _input.LA(1);
					}
					setState(193);
					((JetstoreConfigItemContext)_localctx).declIdentifier = declIdentifier();
					((JetstoreConfigItemContext)_localctx).rdfTypeList.add(((JetstoreConfigItemContext)_localctx).declIdentifier);
					}
					}
					setState(198);
					_errHandler.sync(this);
					_la = _input.LA(1);
				}
				setState(202);
				_errHandler.sync(this);
				_la = _input.LA(1);
				while (_la==COMMENT) {
					{
					{
					setState(199);
					match(COMMENT);
					}
					}
					setState(204);
					_errHandler.sync(this);
					_la = _input.LA(1);
				}
				setState(205);
				match(T__4);
				}
				break;
			default:
				throw new NoViableAltException(this);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class DefineClassStmtContext extends ParserRuleContext {
		public DeclIdentifierContext className;
		public TerminalNode CLASS() { return getToken(JetRuleParser.CLASS, 0); }
		public List<ClassStmtContext> classStmt() {
			return getRuleContexts(ClassStmtContext.class);
		}
		public ClassStmtContext classStmt(int i) {
			return getRuleContext(ClassStmtContext.class,i);
		}
		public TerminalNode SEMICOLON() { return getToken(JetRuleParser.SEMICOLON, 0); }
		public DeclIdentifierContext declIdentifier() {
			return getRuleContext(DeclIdentifierContext.class,0);
		}
		public List<TerminalNode> COMMENT() { return getTokens(JetRuleParser.COMMENT); }
		public TerminalNode COMMENT(int i) {
			return getToken(JetRuleParser.COMMENT, i);
		}
		public DefineClassStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_defineClassStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterDefineClassStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitDefineClassStmt(this);
		}
	}

	public final DefineClassStmtContext defineClassStmt() throws RecognitionException {
		DefineClassStmtContext _localctx = new DefineClassStmtContext(_ctx, getState());
		enterRule(_localctx, 14, RULE_defineClassStmt);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(209);
			match(CLASS);
			setState(210);
			((DefineClassStmtContext)_localctx).className = declIdentifier();
			setState(211);
			match(T__0);
			setState(215);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==COMMENT) {
				{
				{
				setState(212);
				match(COMMENT);
				}
				}
				setState(217);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			setState(218);
			classStmt();
			setState(229);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==T__2) {
				{
				{
				setState(219);
				match(T__2);
				setState(223);
				_errHandler.sync(this);
				_la = _input.LA(1);
				while (_la==COMMENT) {
					{
					{
					setState(220);
					match(COMMENT);
					}
					}
					setState(225);
					_errHandler.sync(this);
					_la = _input.LA(1);
				}
				setState(226);
				classStmt();
				}
				}
				setState(231);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			setState(235);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==COMMENT) {
				{
				{
				setState(232);
				match(COMMENT);
				}
				}
				setState(237);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			setState(238);
			match(T__1);
			setState(239);
			match(SEMICOLON);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class ClassStmtContext extends ParserRuleContext {
		public TerminalNode BaseClasses() { return getToken(JetRuleParser.BaseClasses, 0); }
		public TerminalNode ASSIGN() { return getToken(JetRuleParser.ASSIGN, 0); }
		public List<SubClassOfStmtContext> subClassOfStmt() {
			return getRuleContexts(SubClassOfStmtContext.class);
		}
		public SubClassOfStmtContext subClassOfStmt(int i) {
			return getRuleContext(SubClassOfStmtContext.class,i);
		}
		public List<TerminalNode> COMMENT() { return getTokens(JetRuleParser.COMMENT); }
		public TerminalNode COMMENT(int i) {
			return getToken(JetRuleParser.COMMENT, i);
		}
		public TerminalNode DataProperties() { return getToken(JetRuleParser.DataProperties, 0); }
		public List<DataPropertyDefinitionsContext> dataPropertyDefinitions() {
			return getRuleContexts(DataPropertyDefinitionsContext.class);
		}
		public DataPropertyDefinitionsContext dataPropertyDefinitions(int i) {
			return getRuleContext(DataPropertyDefinitionsContext.class,i);
		}
		public TerminalNode ObjectProperties() { return getToken(JetRuleParser.ObjectProperties, 0); }
		public List<ObjectPropertyDefinitionsContext> objectPropertyDefinitions() {
			return getRuleContexts(ObjectPropertyDefinitionsContext.class);
		}
		public ObjectPropertyDefinitionsContext objectPropertyDefinitions(int i) {
			return getRuleContext(ObjectPropertyDefinitionsContext.class,i);
		}
		public TerminalNode GroupingProperties() { return getToken(JetRuleParser.GroupingProperties, 0); }
		public List<GroupingPropertyStmtContext> groupingPropertyStmt() {
			return getRuleContexts(GroupingPropertyStmtContext.class);
		}
		public GroupingPropertyStmtContext groupingPropertyStmt(int i) {
			return getRuleContext(GroupingPropertyStmtContext.class,i);
		}
		public AsTableStmtContext asTableStmt() {
			return getRuleContext(AsTableStmtContext.class,0);
		}
		public ClassStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_classStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterClassStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitClassStmt(this);
		}
	}

	public final ClassStmtContext classStmt() throws RecognitionException {
		ClassStmtContext _localctx = new ClassStmtContext(_ctx, getState());
		enterRule(_localctx, 16, RULE_classStmt);
		int _la;
		try {
			setState(366);
			_errHandler.sync(this);
			switch (_input.LA(1)) {
			case BaseClasses:
				enterOuterAlt(_localctx, 1);
				{
				setState(241);
				match(BaseClasses);
				setState(242);
				match(ASSIGN);
				setState(243);
				match(T__3);
				setState(247);
				_errHandler.sync(this);
				_la = _input.LA(1);
				while (_la==COMMENT) {
					{
					{
					setState(244);
					match(COMMENT);
					}
					}
					setState(249);
					_errHandler.sync(this);
					_la = _input.LA(1);
				}
				setState(250);
				subClassOfStmt();
				setState(261);
				_errHandler.sync(this);
				_la = _input.LA(1);
				while (_la==T__2) {
					{
					{
					setState(251);
					match(T__2);
					setState(255);
					_errHandler.sync(this);
					_la = _input.LA(1);
					while (_la==COMMENT) {
						{
						{
						setState(252);
						match(COMMENT);
						}
						}
						setState(257);
						_errHandler.sync(this);
						_la = _input.LA(1);
					}
					setState(258);
					subClassOfStmt();
					}
					}
					setState(263);
					_errHandler.sync(this);
					_la = _input.LA(1);
				}
				setState(267);
				_errHandler.sync(this);
				_la = _input.LA(1);
				while (_la==COMMENT) {
					{
					{
					setState(264);
					match(COMMENT);
					}
					}
					setState(269);
					_errHandler.sync(this);
					_la = _input.LA(1);
				}
				setState(270);
				match(T__4);
				}
				break;
			case DataProperties:
				enterOuterAlt(_localctx, 2);
				{
				setState(272);
				match(DataProperties);
				setState(273);
				match(ASSIGN);
				setState(274);
				match(T__3);
				setState(278);
				_errHandler.sync(this);
				_la = _input.LA(1);
				while (_la==COMMENT) {
					{
					{
					setState(275);
					match(COMMENT);
					}
					}
					setState(280);
					_errHandler.sync(this);
					_la = _input.LA(1);
				}
				setState(281);
				dataPropertyDefinitions();
				setState(292);
				_errHandler.sync(this);
				_la = _input.LA(1);
				while (_la==T__2) {
					{
					{
					setState(282);
					match(T__2);
					setState(286);
					_errHandler.sync(this);
					_la = _input.LA(1);
					while (_la==COMMENT) {
						{
						{
						setState(283);
						match(COMMENT);
						}
						}
						setState(288);
						_errHandler.sync(this);
						_la = _input.LA(1);
					}
					setState(289);
					dataPropertyDefinitions();
					}
					}
					setState(294);
					_errHandler.sync(this);
					_la = _input.LA(1);
				}
				setState(298);
				_errHandler.sync(this);
				_la = _input.LA(1);
				while (_la==COMMENT) {
					{
					{
					setState(295);
					match(COMMENT);
					}
					}
					setState(300);
					_errHandler.sync(this);
					_la = _input.LA(1);
				}
				setState(301);
				match(T__4);
				}
				break;
			case ObjectProperties:
				enterOuterAlt(_localctx, 3);
				{
				setState(303);
				match(ObjectProperties);
				setState(304);
				match(ASSIGN);
				setState(305);
				match(T__3);
				setState(309);
				_errHandler.sync(this);
				_la = _input.LA(1);
				while (_la==COMMENT) {
					{
					{
					setState(306);
					match(COMMENT);
					}
					}
					setState(311);
					_errHandler.sync(this);
					_la = _input.LA(1);
				}
				setState(312);
				objectPropertyDefinitions();
				setState(323);
				_errHandler.sync(this);
				_la = _input.LA(1);
				while (_la==T__2) {
					{
					{
					setState(313);
					match(T__2);
					setState(317);
					_errHandler.sync(this);
					_la = _input.LA(1);
					while (_la==COMMENT) {
						{
						{
						setState(314);
						match(COMMENT);
						}
						}
						setState(319);
						_errHandler.sync(this);
						_la = _input.LA(1);
					}
					setState(320);
					objectPropertyDefinitions();
					}
					}
					setState(325);
					_errHandler.sync(this);
					_la = _input.LA(1);
				}
				setState(329);
				_errHandler.sync(this);
				_la = _input.LA(1);
				while (_la==COMMENT) {
					{
					{
					setState(326);
					match(COMMENT);
					}
					}
					setState(331);
					_errHandler.sync(this);
					_la = _input.LA(1);
				}
				setState(332);
				match(T__4);
				}
				break;
			case GroupingProperties:
				enterOuterAlt(_localctx, 4);
				{
				setState(334);
				match(GroupingProperties);
				setState(335);
				match(ASSIGN);
				setState(336);
				match(T__3);
				setState(340);
				_errHandler.sync(this);
				_la = _input.LA(1);
				while (_la==COMMENT) {
					{
					{
					setState(337);
					match(COMMENT);
					}
					}
					setState(342);
					_errHandler.sync(this);
					_la = _input.LA(1);
				}
				setState(343);
				groupingPropertyStmt();
				setState(354);
				_errHandler.sync(this);
				_la = _input.LA(1);
				while (_la==T__2) {
					{
					{
					setState(344);
					match(T__2);
					setState(348);
					_errHandler.sync(this);
					_la = _input.LA(1);
					while (_la==COMMENT) {
						{
						{
						setState(345);
						match(COMMENT);
						}
						}
						setState(350);
						_errHandler.sync(this);
						_la = _input.LA(1);
					}
					setState(351);
					groupingPropertyStmt();
					}
					}
					setState(356);
					_errHandler.sync(this);
					_la = _input.LA(1);
				}
				setState(360);
				_errHandler.sync(this);
				_la = _input.LA(1);
				while (_la==COMMENT) {
					{
					{
					setState(357);
					match(COMMENT);
					}
					}
					setState(362);
					_errHandler.sync(this);
					_la = _input.LA(1);
				}
				setState(363);
				match(T__4);
				}
				break;
			case AsTable:
				enterOuterAlt(_localctx, 5);
				{
				setState(365);
				asTableStmt();
				}
				break;
			default:
				throw new NoViableAltException(this);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class SubClassOfStmtContext extends ParserRuleContext {
		public DeclIdentifierContext baseClassName;
		public DeclIdentifierContext declIdentifier() {
			return getRuleContext(DeclIdentifierContext.class,0);
		}
		public SubClassOfStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_subClassOfStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterSubClassOfStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitSubClassOfStmt(this);
		}
	}

	public final SubClassOfStmtContext subClassOfStmt() throws RecognitionException {
		SubClassOfStmtContext _localctx = new SubClassOfStmtContext(_ctx, getState());
		enterRule(_localctx, 18, RULE_subClassOfStmt);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(368);
			((SubClassOfStmtContext)_localctx).baseClassName = declIdentifier();
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class DataPropertyDefinitionsContext extends ParserRuleContext {
		public DeclIdentifierContext dataPName;
		public Token array;
		public DataPropertyTypeContext dataPType;
		public DeclIdentifierContext declIdentifier() {
			return getRuleContext(DeclIdentifierContext.class,0);
		}
		public DataPropertyTypeContext dataPropertyType() {
			return getRuleContext(DataPropertyTypeContext.class,0);
		}
		public TerminalNode ARRAY() { return getToken(JetRuleParser.ARRAY, 0); }
		public DataPropertyDefinitionsContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_dataPropertyDefinitions; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterDataPropertyDefinitions(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitDataPropertyDefinitions(this);
		}
	}

	public final DataPropertyDefinitionsContext dataPropertyDefinitions() throws RecognitionException {
		DataPropertyDefinitionsContext _localctx = new DataPropertyDefinitionsContext(_ctx, getState());
		enterRule(_localctx, 20, RULE_dataPropertyDefinitions);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(370);
			((DataPropertyDefinitionsContext)_localctx).dataPName = declIdentifier();
			setState(371);
			match(T__5);
			setState(373);
			_errHandler.sync(this);
			_la = _input.LA(1);
			if (_la==ARRAY) {
				{
				setState(372);
				((DataPropertyDefinitionsContext)_localctx).array = match(ARRAY);
				}
			}

			setState(375);
			((DataPropertyDefinitionsContext)_localctx).dataPType = dataPropertyType();
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class ObjectPropertyDefinitionsContext extends ParserRuleContext {
		public DeclIdentifierContext objectPName;
		public Token array;
		public TerminalNode ResourceType() { return getToken(JetRuleParser.ResourceType, 0); }
		public DeclIdentifierContext declIdentifier() {
			return getRuleContext(DeclIdentifierContext.class,0);
		}
		public TerminalNode ARRAY() { return getToken(JetRuleParser.ARRAY, 0); }
		public ObjectPropertyDefinitionsContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_objectPropertyDefinitions; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterObjectPropertyDefinitions(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitObjectPropertyDefinitions(this);
		}
	}

	public final ObjectPropertyDefinitionsContext objectPropertyDefinitions() throws RecognitionException {
		ObjectPropertyDefinitionsContext _localctx = new ObjectPropertyDefinitionsContext(_ctx, getState());
		enterRule(_localctx, 22, RULE_objectPropertyDefinitions);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(377);
			((ObjectPropertyDefinitionsContext)_localctx).objectPName = declIdentifier();
			setState(378);
			match(T__5);
			setState(380);
			_errHandler.sync(this);
			_la = _input.LA(1);
			if (_la==ARRAY) {
				{
				setState(379);
				((ObjectPropertyDefinitionsContext)_localctx).array = match(ARRAY);
				}
			}

			setState(382);
			match(ResourceType);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class DataPropertyTypeContext extends ParserRuleContext {
		public TerminalNode Int32Type() { return getToken(JetRuleParser.Int32Type, 0); }
		public TerminalNode UInt32Type() { return getToken(JetRuleParser.UInt32Type, 0); }
		public TerminalNode Int64Type() { return getToken(JetRuleParser.Int64Type, 0); }
		public TerminalNode UInt64Type() { return getToken(JetRuleParser.UInt64Type, 0); }
		public TerminalNode DoubleType() { return getToken(JetRuleParser.DoubleType, 0); }
		public TerminalNode StringType() { return getToken(JetRuleParser.StringType, 0); }
		public TerminalNode DateType() { return getToken(JetRuleParser.DateType, 0); }
		public TerminalNode DatetimeType() { return getToken(JetRuleParser.DatetimeType, 0); }
		public TerminalNode BoolType() { return getToken(JetRuleParser.BoolType, 0); }
		public TerminalNode ResourceType() { return getToken(JetRuleParser.ResourceType, 0); }
		public DataPropertyTypeContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_dataPropertyType; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterDataPropertyType(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitDataPropertyType(this);
		}
	}

	public final DataPropertyTypeContext dataPropertyType() throws RecognitionException {
		DataPropertyTypeContext _localctx = new DataPropertyTypeContext(_ctx, getState());
		enterRule(_localctx, 24, RULE_dataPropertyType);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(384);
			_la = _input.LA(1);
			if ( !((((_la) & ~0x3f) == 0 && ((1L << _la) & 549218942976L) != 0)) ) {
			_errHandler.recoverInline(this);
			}
			else {
				if ( _input.LA(1)==Token.EOF ) matchedEOF = true;
				_errHandler.reportMatch(this);
				consume();
			}
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class GroupingPropertyStmtContext extends ParserRuleContext {
		public DeclIdentifierContext groupingPropertyName;
		public DeclIdentifierContext declIdentifier() {
			return getRuleContext(DeclIdentifierContext.class,0);
		}
		public GroupingPropertyStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_groupingPropertyStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterGroupingPropertyStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitGroupingPropertyStmt(this);
		}
	}

	public final GroupingPropertyStmtContext groupingPropertyStmt() throws RecognitionException {
		GroupingPropertyStmtContext _localctx = new GroupingPropertyStmtContext(_ctx, getState());
		enterRule(_localctx, 26, RULE_groupingPropertyStmt);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(386);
			((GroupingPropertyStmtContext)_localctx).groupingPropertyName = declIdentifier();
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class AsTableStmtContext extends ParserRuleContext {
		public AsTableFlagContext asTable;
		public TerminalNode AsTable() { return getToken(JetRuleParser.AsTable, 0); }
		public TerminalNode ASSIGN() { return getToken(JetRuleParser.ASSIGN, 0); }
		public AsTableFlagContext asTableFlag() {
			return getRuleContext(AsTableFlagContext.class,0);
		}
		public AsTableStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_asTableStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterAsTableStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitAsTableStmt(this);
		}
	}

	public final AsTableStmtContext asTableStmt() throws RecognitionException {
		AsTableStmtContext _localctx = new AsTableStmtContext(_ctx, getState());
		enterRule(_localctx, 28, RULE_asTableStmt);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(388);
			match(AsTable);
			setState(389);
			match(ASSIGN);
			setState(390);
			((AsTableStmtContext)_localctx).asTable = asTableFlag();
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class AsTableFlagContext extends ParserRuleContext {
		public TerminalNode TRUE() { return getToken(JetRuleParser.TRUE, 0); }
		public TerminalNode FALSE() { return getToken(JetRuleParser.FALSE, 0); }
		public AsTableFlagContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_asTableFlag; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterAsTableFlag(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitAsTableFlag(this);
		}
	}

	public final AsTableFlagContext asTableFlag() throws RecognitionException {
		AsTableFlagContext _localctx = new AsTableFlagContext(_ctx, getState());
		enterRule(_localctx, 30, RULE_asTableFlag);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(392);
			_la = _input.LA(1);
			if ( !(_la==TRUE || _la==FALSE) ) {
			_errHandler.recoverInline(this);
			}
			else {
				if ( _input.LA(1)==Token.EOF ) matchedEOF = true;
				_errHandler.reportMatch(this);
				consume();
			}
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class DefineRuleSeqStmtContext extends ParserRuleContext {
		public Token ruleseqName;
		public TerminalNode RULESEQ() { return getToken(JetRuleParser.RULESEQ, 0); }
		public TerminalNode MainRuleSets() { return getToken(JetRuleParser.MainRuleSets, 0); }
		public TerminalNode ASSIGN() { return getToken(JetRuleParser.ASSIGN, 0); }
		public RuleSetSeqContext ruleSetSeq() {
			return getRuleContext(RuleSetSeqContext.class,0);
		}
		public TerminalNode SEMICOLON() { return getToken(JetRuleParser.SEMICOLON, 0); }
		public TerminalNode Identifier() { return getToken(JetRuleParser.Identifier, 0); }
		public List<TerminalNode> COMMENT() { return getTokens(JetRuleParser.COMMENT); }
		public TerminalNode COMMENT(int i) {
			return getToken(JetRuleParser.COMMENT, i);
		}
		public DefineRuleSeqStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_defineRuleSeqStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterDefineRuleSeqStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitDefineRuleSeqStmt(this);
		}
	}

	public final DefineRuleSeqStmtContext defineRuleSeqStmt() throws RecognitionException {
		DefineRuleSeqStmtContext _localctx = new DefineRuleSeqStmtContext(_ctx, getState());
		enterRule(_localctx, 32, RULE_defineRuleSeqStmt);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(394);
			match(RULESEQ);
			setState(395);
			((DefineRuleSeqStmtContext)_localctx).ruleseqName = match(Identifier);
			setState(396);
			match(T__0);
			setState(400);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==COMMENT) {
				{
				{
				setState(397);
				match(COMMENT);
				}
				}
				setState(402);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			setState(403);
			match(MainRuleSets);
			setState(404);
			match(ASSIGN);
			setState(405);
			match(T__3);
			setState(409);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==COMMENT) {
				{
				{
				setState(406);
				match(COMMENT);
				}
				}
				setState(411);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			setState(412);
			ruleSetSeq();
			setState(416);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==COMMENT) {
				{
				{
				setState(413);
				match(COMMENT);
				}
				}
				setState(418);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			setState(419);
			match(T__4);
			setState(421);
			_errHandler.sync(this);
			_la = _input.LA(1);
			if (_la==T__2) {
				{
				setState(420);
				match(T__2);
				}
			}

			setState(426);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==COMMENT) {
				{
				{
				setState(423);
				match(COMMENT);
				}
				}
				setState(428);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			setState(429);
			match(T__1);
			setState(430);
			match(SEMICOLON);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class RuleSetSeqContext extends ParserRuleContext {
		public List<RuleSetDefinitionsContext> ruleSetDefinitions() {
			return getRuleContexts(RuleSetDefinitionsContext.class);
		}
		public RuleSetDefinitionsContext ruleSetDefinitions(int i) {
			return getRuleContext(RuleSetDefinitionsContext.class,i);
		}
		public List<TerminalNode> COMMENT() { return getTokens(JetRuleParser.COMMENT); }
		public TerminalNode COMMENT(int i) {
			return getToken(JetRuleParser.COMMENT, i);
		}
		public RuleSetSeqContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_ruleSetSeq; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterRuleSetSeq(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitRuleSetSeq(this);
		}
	}

	public final RuleSetSeqContext ruleSetSeq() throws RecognitionException {
		RuleSetSeqContext _localctx = new RuleSetSeqContext(_ctx, getState());
		enterRule(_localctx, 34, RULE_ruleSetSeq);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(432);
			ruleSetDefinitions();
			setState(443);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==T__2) {
				{
				{
				setState(433);
				match(T__2);
				setState(437);
				_errHandler.sync(this);
				_la = _input.LA(1);
				while (_la==COMMENT) {
					{
					{
					setState(434);
					match(COMMENT);
					}
					}
					setState(439);
					_errHandler.sync(this);
					_la = _input.LA(1);
				}
				setState(440);
				ruleSetDefinitions();
				}
				}
				setState(445);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class RuleSetDefinitionsContext extends ParserRuleContext {
		public Token rsName;
		public TerminalNode STRING() { return getToken(JetRuleParser.STRING, 0); }
		public RuleSetDefinitionsContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_ruleSetDefinitions; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterRuleSetDefinitions(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitRuleSetDefinitions(this);
		}
	}

	public final RuleSetDefinitionsContext ruleSetDefinitions() throws RecognitionException {
		RuleSetDefinitionsContext _localctx = new RuleSetDefinitionsContext(_ctx, getState());
		enterRule(_localctx, 36, RULE_ruleSetDefinitions);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(446);
			((RuleSetDefinitionsContext)_localctx).rsName = match(STRING);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class DefineLiteralStmtContext extends ParserRuleContext {
		public Int32LiteralStmtContext int32LiteralStmt() {
			return getRuleContext(Int32LiteralStmtContext.class,0);
		}
		public UInt32LiteralStmtContext uInt32LiteralStmt() {
			return getRuleContext(UInt32LiteralStmtContext.class,0);
		}
		public Int64LiteralStmtContext int64LiteralStmt() {
			return getRuleContext(Int64LiteralStmtContext.class,0);
		}
		public UInt64LiteralStmtContext uInt64LiteralStmt() {
			return getRuleContext(UInt64LiteralStmtContext.class,0);
		}
		public DoubleLiteralStmtContext doubleLiteralStmt() {
			return getRuleContext(DoubleLiteralStmtContext.class,0);
		}
		public StringLiteralStmtContext stringLiteralStmt() {
			return getRuleContext(StringLiteralStmtContext.class,0);
		}
		public DateLiteralStmtContext dateLiteralStmt() {
			return getRuleContext(DateLiteralStmtContext.class,0);
		}
		public DatetimeLiteralStmtContext datetimeLiteralStmt() {
			return getRuleContext(DatetimeLiteralStmtContext.class,0);
		}
		public BooleanLiteralStmtContext booleanLiteralStmt() {
			return getRuleContext(BooleanLiteralStmtContext.class,0);
		}
		public DefineLiteralStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_defineLiteralStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterDefineLiteralStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitDefineLiteralStmt(this);
		}
	}

	public final DefineLiteralStmtContext defineLiteralStmt() throws RecognitionException {
		DefineLiteralStmtContext _localctx = new DefineLiteralStmtContext(_ctx, getState());
		enterRule(_localctx, 38, RULE_defineLiteralStmt);
		try {
			setState(457);
			_errHandler.sync(this);
			switch (_input.LA(1)) {
			case Int32Type:
				enterOuterAlt(_localctx, 1);
				{
				setState(448);
				int32LiteralStmt();
				}
				break;
			case UInt32Type:
				enterOuterAlt(_localctx, 2);
				{
				setState(449);
				uInt32LiteralStmt();
				}
				break;
			case Int64Type:
				enterOuterAlt(_localctx, 3);
				{
				setState(450);
				int64LiteralStmt();
				}
				break;
			case UInt64Type:
				enterOuterAlt(_localctx, 4);
				{
				setState(451);
				uInt64LiteralStmt();
				}
				break;
			case DoubleType:
				enterOuterAlt(_localctx, 5);
				{
				setState(452);
				doubleLiteralStmt();
				}
				break;
			case StringType:
				enterOuterAlt(_localctx, 6);
				{
				setState(453);
				stringLiteralStmt();
				}
				break;
			case DateType:
				enterOuterAlt(_localctx, 7);
				{
				setState(454);
				dateLiteralStmt();
				}
				break;
			case DatetimeType:
				enterOuterAlt(_localctx, 8);
				{
				setState(455);
				datetimeLiteralStmt();
				}
				break;
			case BoolType:
				enterOuterAlt(_localctx, 9);
				{
				setState(456);
				booleanLiteralStmt();
				}
				break;
			default:
				throw new NoViableAltException(this);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class Int32LiteralStmtContext extends ParserRuleContext {
		public Token varType;
		public DeclIdentifierContext varName;
		public IntExprContext declValue;
		public TerminalNode ASSIGN() { return getToken(JetRuleParser.ASSIGN, 0); }
		public TerminalNode SEMICOLON() { return getToken(JetRuleParser.SEMICOLON, 0); }
		public TerminalNode Int32Type() { return getToken(JetRuleParser.Int32Type, 0); }
		public DeclIdentifierContext declIdentifier() {
			return getRuleContext(DeclIdentifierContext.class,0);
		}
		public IntExprContext intExpr() {
			return getRuleContext(IntExprContext.class,0);
		}
		public Int32LiteralStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_int32LiteralStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterInt32LiteralStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitInt32LiteralStmt(this);
		}
	}

	public final Int32LiteralStmtContext int32LiteralStmt() throws RecognitionException {
		Int32LiteralStmtContext _localctx = new Int32LiteralStmtContext(_ctx, getState());
		enterRule(_localctx, 40, RULE_int32LiteralStmt);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(459);
			((Int32LiteralStmtContext)_localctx).varType = match(Int32Type);
			setState(460);
			((Int32LiteralStmtContext)_localctx).varName = declIdentifier();
			setState(461);
			match(ASSIGN);
			setState(462);
			((Int32LiteralStmtContext)_localctx).declValue = intExpr();
			setState(463);
			match(SEMICOLON);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class UInt32LiteralStmtContext extends ParserRuleContext {
		public Token varType;
		public DeclIdentifierContext varName;
		public UintExprContext declValue;
		public TerminalNode ASSIGN() { return getToken(JetRuleParser.ASSIGN, 0); }
		public TerminalNode SEMICOLON() { return getToken(JetRuleParser.SEMICOLON, 0); }
		public TerminalNode UInt32Type() { return getToken(JetRuleParser.UInt32Type, 0); }
		public DeclIdentifierContext declIdentifier() {
			return getRuleContext(DeclIdentifierContext.class,0);
		}
		public UintExprContext uintExpr() {
			return getRuleContext(UintExprContext.class,0);
		}
		public UInt32LiteralStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_uInt32LiteralStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterUInt32LiteralStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitUInt32LiteralStmt(this);
		}
	}

	public final UInt32LiteralStmtContext uInt32LiteralStmt() throws RecognitionException {
		UInt32LiteralStmtContext _localctx = new UInt32LiteralStmtContext(_ctx, getState());
		enterRule(_localctx, 42, RULE_uInt32LiteralStmt);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(465);
			((UInt32LiteralStmtContext)_localctx).varType = match(UInt32Type);
			setState(466);
			((UInt32LiteralStmtContext)_localctx).varName = declIdentifier();
			setState(467);
			match(ASSIGN);
			setState(468);
			((UInt32LiteralStmtContext)_localctx).declValue = uintExpr();
			setState(469);
			match(SEMICOLON);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class Int64LiteralStmtContext extends ParserRuleContext {
		public Token varType;
		public DeclIdentifierContext varName;
		public IntExprContext declValue;
		public TerminalNode ASSIGN() { return getToken(JetRuleParser.ASSIGN, 0); }
		public TerminalNode SEMICOLON() { return getToken(JetRuleParser.SEMICOLON, 0); }
		public TerminalNode Int64Type() { return getToken(JetRuleParser.Int64Type, 0); }
		public DeclIdentifierContext declIdentifier() {
			return getRuleContext(DeclIdentifierContext.class,0);
		}
		public IntExprContext intExpr() {
			return getRuleContext(IntExprContext.class,0);
		}
		public Int64LiteralStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_int64LiteralStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterInt64LiteralStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitInt64LiteralStmt(this);
		}
	}

	public final Int64LiteralStmtContext int64LiteralStmt() throws RecognitionException {
		Int64LiteralStmtContext _localctx = new Int64LiteralStmtContext(_ctx, getState());
		enterRule(_localctx, 44, RULE_int64LiteralStmt);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(471);
			((Int64LiteralStmtContext)_localctx).varType = match(Int64Type);
			setState(472);
			((Int64LiteralStmtContext)_localctx).varName = declIdentifier();
			setState(473);
			match(ASSIGN);
			setState(474);
			((Int64LiteralStmtContext)_localctx).declValue = intExpr();
			setState(475);
			match(SEMICOLON);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class UInt64LiteralStmtContext extends ParserRuleContext {
		public Token varType;
		public DeclIdentifierContext varName;
		public UintExprContext declValue;
		public TerminalNode ASSIGN() { return getToken(JetRuleParser.ASSIGN, 0); }
		public TerminalNode SEMICOLON() { return getToken(JetRuleParser.SEMICOLON, 0); }
		public TerminalNode UInt64Type() { return getToken(JetRuleParser.UInt64Type, 0); }
		public DeclIdentifierContext declIdentifier() {
			return getRuleContext(DeclIdentifierContext.class,0);
		}
		public UintExprContext uintExpr() {
			return getRuleContext(UintExprContext.class,0);
		}
		public UInt64LiteralStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_uInt64LiteralStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterUInt64LiteralStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitUInt64LiteralStmt(this);
		}
	}

	public final UInt64LiteralStmtContext uInt64LiteralStmt() throws RecognitionException {
		UInt64LiteralStmtContext _localctx = new UInt64LiteralStmtContext(_ctx, getState());
		enterRule(_localctx, 46, RULE_uInt64LiteralStmt);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(477);
			((UInt64LiteralStmtContext)_localctx).varType = match(UInt64Type);
			setState(478);
			((UInt64LiteralStmtContext)_localctx).varName = declIdentifier();
			setState(479);
			match(ASSIGN);
			setState(480);
			((UInt64LiteralStmtContext)_localctx).declValue = uintExpr();
			setState(481);
			match(SEMICOLON);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class DoubleLiteralStmtContext extends ParserRuleContext {
		public Token varType;
		public DeclIdentifierContext varName;
		public DoubleExprContext declValue;
		public TerminalNode ASSIGN() { return getToken(JetRuleParser.ASSIGN, 0); }
		public TerminalNode SEMICOLON() { return getToken(JetRuleParser.SEMICOLON, 0); }
		public TerminalNode DoubleType() { return getToken(JetRuleParser.DoubleType, 0); }
		public DeclIdentifierContext declIdentifier() {
			return getRuleContext(DeclIdentifierContext.class,0);
		}
		public DoubleExprContext doubleExpr() {
			return getRuleContext(DoubleExprContext.class,0);
		}
		public DoubleLiteralStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_doubleLiteralStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterDoubleLiteralStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitDoubleLiteralStmt(this);
		}
	}

	public final DoubleLiteralStmtContext doubleLiteralStmt() throws RecognitionException {
		DoubleLiteralStmtContext _localctx = new DoubleLiteralStmtContext(_ctx, getState());
		enterRule(_localctx, 48, RULE_doubleLiteralStmt);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(483);
			((DoubleLiteralStmtContext)_localctx).varType = match(DoubleType);
			setState(484);
			((DoubleLiteralStmtContext)_localctx).varName = declIdentifier();
			setState(485);
			match(ASSIGN);
			setState(486);
			((DoubleLiteralStmtContext)_localctx).declValue = doubleExpr();
			setState(487);
			match(SEMICOLON);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class StringLiteralStmtContext extends ParserRuleContext {
		public Token varType;
		public DeclIdentifierContext varName;
		public Token declValue;
		public TerminalNode ASSIGN() { return getToken(JetRuleParser.ASSIGN, 0); }
		public TerminalNode SEMICOLON() { return getToken(JetRuleParser.SEMICOLON, 0); }
		public TerminalNode StringType() { return getToken(JetRuleParser.StringType, 0); }
		public DeclIdentifierContext declIdentifier() {
			return getRuleContext(DeclIdentifierContext.class,0);
		}
		public TerminalNode STRING() { return getToken(JetRuleParser.STRING, 0); }
		public StringLiteralStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_stringLiteralStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterStringLiteralStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitStringLiteralStmt(this);
		}
	}

	public final StringLiteralStmtContext stringLiteralStmt() throws RecognitionException {
		StringLiteralStmtContext _localctx = new StringLiteralStmtContext(_ctx, getState());
		enterRule(_localctx, 50, RULE_stringLiteralStmt);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(489);
			((StringLiteralStmtContext)_localctx).varType = match(StringType);
			setState(490);
			((StringLiteralStmtContext)_localctx).varName = declIdentifier();
			setState(491);
			match(ASSIGN);
			setState(492);
			((StringLiteralStmtContext)_localctx).declValue = match(STRING);
			setState(493);
			match(SEMICOLON);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class DateLiteralStmtContext extends ParserRuleContext {
		public Token varType;
		public DeclIdentifierContext varName;
		public Token declValue;
		public TerminalNode ASSIGN() { return getToken(JetRuleParser.ASSIGN, 0); }
		public TerminalNode SEMICOLON() { return getToken(JetRuleParser.SEMICOLON, 0); }
		public TerminalNode DateType() { return getToken(JetRuleParser.DateType, 0); }
		public DeclIdentifierContext declIdentifier() {
			return getRuleContext(DeclIdentifierContext.class,0);
		}
		public TerminalNode STRING() { return getToken(JetRuleParser.STRING, 0); }
		public DateLiteralStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_dateLiteralStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterDateLiteralStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitDateLiteralStmt(this);
		}
	}

	public final DateLiteralStmtContext dateLiteralStmt() throws RecognitionException {
		DateLiteralStmtContext _localctx = new DateLiteralStmtContext(_ctx, getState());
		enterRule(_localctx, 52, RULE_dateLiteralStmt);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(495);
			((DateLiteralStmtContext)_localctx).varType = match(DateType);
			setState(496);
			((DateLiteralStmtContext)_localctx).varName = declIdentifier();
			setState(497);
			match(ASSIGN);
			setState(498);
			((DateLiteralStmtContext)_localctx).declValue = match(STRING);
			setState(499);
			match(SEMICOLON);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class DatetimeLiteralStmtContext extends ParserRuleContext {
		public Token varType;
		public DeclIdentifierContext varName;
		public Token declValue;
		public TerminalNode ASSIGN() { return getToken(JetRuleParser.ASSIGN, 0); }
		public TerminalNode SEMICOLON() { return getToken(JetRuleParser.SEMICOLON, 0); }
		public TerminalNode DatetimeType() { return getToken(JetRuleParser.DatetimeType, 0); }
		public DeclIdentifierContext declIdentifier() {
			return getRuleContext(DeclIdentifierContext.class,0);
		}
		public TerminalNode STRING() { return getToken(JetRuleParser.STRING, 0); }
		public DatetimeLiteralStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_datetimeLiteralStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterDatetimeLiteralStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitDatetimeLiteralStmt(this);
		}
	}

	public final DatetimeLiteralStmtContext datetimeLiteralStmt() throws RecognitionException {
		DatetimeLiteralStmtContext _localctx = new DatetimeLiteralStmtContext(_ctx, getState());
		enterRule(_localctx, 54, RULE_datetimeLiteralStmt);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(501);
			((DatetimeLiteralStmtContext)_localctx).varType = match(DatetimeType);
			setState(502);
			((DatetimeLiteralStmtContext)_localctx).varName = declIdentifier();
			setState(503);
			match(ASSIGN);
			setState(504);
			((DatetimeLiteralStmtContext)_localctx).declValue = match(STRING);
			setState(505);
			match(SEMICOLON);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class BooleanLiteralStmtContext extends ParserRuleContext {
		public Token varType;
		public DeclIdentifierContext varName;
		public Token declValue;
		public TerminalNode ASSIGN() { return getToken(JetRuleParser.ASSIGN, 0); }
		public TerminalNode SEMICOLON() { return getToken(JetRuleParser.SEMICOLON, 0); }
		public TerminalNode BoolType() { return getToken(JetRuleParser.BoolType, 0); }
		public DeclIdentifierContext declIdentifier() {
			return getRuleContext(DeclIdentifierContext.class,0);
		}
		public TerminalNode STRING() { return getToken(JetRuleParser.STRING, 0); }
		public BooleanLiteralStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_booleanLiteralStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterBooleanLiteralStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitBooleanLiteralStmt(this);
		}
	}

	public final BooleanLiteralStmtContext booleanLiteralStmt() throws RecognitionException {
		BooleanLiteralStmtContext _localctx = new BooleanLiteralStmtContext(_ctx, getState());
		enterRule(_localctx, 56, RULE_booleanLiteralStmt);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(507);
			((BooleanLiteralStmtContext)_localctx).varType = match(BoolType);
			setState(508);
			((BooleanLiteralStmtContext)_localctx).varName = declIdentifier();
			setState(509);
			match(ASSIGN);
			setState(510);
			((BooleanLiteralStmtContext)_localctx).declValue = match(STRING);
			setState(511);
			match(SEMICOLON);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class IntExprContext extends ParserRuleContext {
		public TerminalNode PLUS() { return getToken(JetRuleParser.PLUS, 0); }
		public IntExprContext intExpr() {
			return getRuleContext(IntExprContext.class,0);
		}
		public TerminalNode MINUS() { return getToken(JetRuleParser.MINUS, 0); }
		public TerminalNode DIGITS() { return getToken(JetRuleParser.DIGITS, 0); }
		public IntExprContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_intExpr; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterIntExpr(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitIntExpr(this);
		}
	}

	public final IntExprContext intExpr() throws RecognitionException {
		IntExprContext _localctx = new IntExprContext(_ctx, getState());
		enterRule(_localctx, 58, RULE_intExpr);
		try {
			setState(518);
			_errHandler.sync(this);
			switch (_input.LA(1)) {
			case PLUS:
				enterOuterAlt(_localctx, 1);
				{
				setState(513);
				match(PLUS);
				setState(514);
				intExpr();
				}
				break;
			case MINUS:
				enterOuterAlt(_localctx, 2);
				{
				setState(515);
				match(MINUS);
				setState(516);
				intExpr();
				}
				break;
			case DIGITS:
				enterOuterAlt(_localctx, 3);
				{
				setState(517);
				match(DIGITS);
				}
				break;
			default:
				throw new NoViableAltException(this);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class UintExprContext extends ParserRuleContext {
		public TerminalNode PLUS() { return getToken(JetRuleParser.PLUS, 0); }
		public UintExprContext uintExpr() {
			return getRuleContext(UintExprContext.class,0);
		}
		public TerminalNode DIGITS() { return getToken(JetRuleParser.DIGITS, 0); }
		public UintExprContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_uintExpr; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterUintExpr(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitUintExpr(this);
		}
	}

	public final UintExprContext uintExpr() throws RecognitionException {
		UintExprContext _localctx = new UintExprContext(_ctx, getState());
		enterRule(_localctx, 60, RULE_uintExpr);
		try {
			setState(523);
			_errHandler.sync(this);
			switch (_input.LA(1)) {
			case PLUS:
				enterOuterAlt(_localctx, 1);
				{
				setState(520);
				match(PLUS);
				setState(521);
				uintExpr();
				}
				break;
			case DIGITS:
				enterOuterAlt(_localctx, 2);
				{
				setState(522);
				match(DIGITS);
				}
				break;
			default:
				throw new NoViableAltException(this);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class DoubleExprContext extends ParserRuleContext {
		public TerminalNode PLUS() { return getToken(JetRuleParser.PLUS, 0); }
		public DoubleExprContext doubleExpr() {
			return getRuleContext(DoubleExprContext.class,0);
		}
		public TerminalNode MINUS() { return getToken(JetRuleParser.MINUS, 0); }
		public List<TerminalNode> DIGITS() { return getTokens(JetRuleParser.DIGITS); }
		public TerminalNode DIGITS(int i) {
			return getToken(JetRuleParser.DIGITS, i);
		}
		public DoubleExprContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_doubleExpr; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterDoubleExpr(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitDoubleExpr(this);
		}
	}

	public final DoubleExprContext doubleExpr() throws RecognitionException {
		DoubleExprContext _localctx = new DoubleExprContext(_ctx, getState());
		enterRule(_localctx, 62, RULE_doubleExpr);
		try {
			setState(534);
			_errHandler.sync(this);
			switch (_input.LA(1)) {
			case PLUS:
				enterOuterAlt(_localctx, 1);
				{
				setState(525);
				match(PLUS);
				setState(526);
				doubleExpr();
				}
				break;
			case MINUS:
				enterOuterAlt(_localctx, 2);
				{
				setState(527);
				match(MINUS);
				setState(528);
				doubleExpr();
				}
				break;
			case DIGITS:
				enterOuterAlt(_localctx, 3);
				{
				setState(529);
				match(DIGITS);
				setState(532);
				_errHandler.sync(this);
				switch ( getInterpreter().adaptivePredict(_input,44,_ctx) ) {
				case 1:
					{
					setState(530);
					match(T__6);
					setState(531);
					match(DIGITS);
					}
					break;
				}
				}
				break;
			default:
				throw new NoViableAltException(this);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class DeclIdentifierContext extends ParserRuleContext {
		public List<TerminalNode> Identifier() { return getTokens(JetRuleParser.Identifier); }
		public TerminalNode Identifier(int i) {
			return getToken(JetRuleParser.Identifier, i);
		}
		public TerminalNode STRING() { return getToken(JetRuleParser.STRING, 0); }
		public DeclIdentifierContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_declIdentifier; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterDeclIdentifier(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitDeclIdentifier(this);
		}
	}

	public final DeclIdentifierContext declIdentifier() throws RecognitionException {
		DeclIdentifierContext _localctx = new DeclIdentifierContext(_ctx, getState());
		enterRule(_localctx, 64, RULE_declIdentifier);
		try {
			setState(543);
			_errHandler.sync(this);
			switch ( getInterpreter().adaptivePredict(_input,46,_ctx) ) {
			case 1:
				enterOuterAlt(_localctx, 1);
				{
				setState(536);
				match(Identifier);
				setState(537);
				match(T__7);
				setState(538);
				match(Identifier);
				}
				break;
			case 2:
				enterOuterAlt(_localctx, 2);
				{
				setState(539);
				match(Identifier);
				setState(540);
				match(T__7);
				setState(541);
				match(STRING);
				}
				break;
			case 3:
				enterOuterAlt(_localctx, 3);
				{
				setState(542);
				match(Identifier);
				}
				break;
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class DefineResourceStmtContext extends ParserRuleContext {
		public NamedResourceStmtContext namedResourceStmt() {
			return getRuleContext(NamedResourceStmtContext.class,0);
		}
		public VolatileResourceStmtContext volatileResourceStmt() {
			return getRuleContext(VolatileResourceStmtContext.class,0);
		}
		public DefineResourceStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_defineResourceStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterDefineResourceStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitDefineResourceStmt(this);
		}
	}

	public final DefineResourceStmtContext defineResourceStmt() throws RecognitionException {
		DefineResourceStmtContext _localctx = new DefineResourceStmtContext(_ctx, getState());
		enterRule(_localctx, 66, RULE_defineResourceStmt);
		try {
			setState(547);
			_errHandler.sync(this);
			switch (_input.LA(1)) {
			case ResourceType:
				enterOuterAlt(_localctx, 1);
				{
				setState(545);
				namedResourceStmt();
				}
				break;
			case VolatileResourceType:
				enterOuterAlt(_localctx, 2);
				{
				setState(546);
				volatileResourceStmt();
				}
				break;
			default:
				throw new NoViableAltException(this);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class NamedResourceStmtContext extends ParserRuleContext {
		public DeclIdentifierContext resName;
		public ResourceValueContext resCtx;
		public TerminalNode ResourceType() { return getToken(JetRuleParser.ResourceType, 0); }
		public TerminalNode ASSIGN() { return getToken(JetRuleParser.ASSIGN, 0); }
		public TerminalNode SEMICOLON() { return getToken(JetRuleParser.SEMICOLON, 0); }
		public DeclIdentifierContext declIdentifier() {
			return getRuleContext(DeclIdentifierContext.class,0);
		}
		public ResourceValueContext resourceValue() {
			return getRuleContext(ResourceValueContext.class,0);
		}
		public NamedResourceStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_namedResourceStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterNamedResourceStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitNamedResourceStmt(this);
		}
	}

	public final NamedResourceStmtContext namedResourceStmt() throws RecognitionException {
		NamedResourceStmtContext _localctx = new NamedResourceStmtContext(_ctx, getState());
		enterRule(_localctx, 68, RULE_namedResourceStmt);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(549);
			match(ResourceType);
			setState(550);
			((NamedResourceStmtContext)_localctx).resName = declIdentifier();
			setState(551);
			match(ASSIGN);
			setState(552);
			((NamedResourceStmtContext)_localctx).resCtx = resourceValue();
			setState(553);
			match(SEMICOLON);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class VolatileResourceStmtContext extends ParserRuleContext {
		public Token resType;
		public DeclIdentifierContext resName;
		public Token resVal;
		public TerminalNode ASSIGN() { return getToken(JetRuleParser.ASSIGN, 0); }
		public TerminalNode SEMICOLON() { return getToken(JetRuleParser.SEMICOLON, 0); }
		public TerminalNode VolatileResourceType() { return getToken(JetRuleParser.VolatileResourceType, 0); }
		public DeclIdentifierContext declIdentifier() {
			return getRuleContext(DeclIdentifierContext.class,0);
		}
		public TerminalNode STRING() { return getToken(JetRuleParser.STRING, 0); }
		public VolatileResourceStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_volatileResourceStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterVolatileResourceStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitVolatileResourceStmt(this);
		}
	}

	public final VolatileResourceStmtContext volatileResourceStmt() throws RecognitionException {
		VolatileResourceStmtContext _localctx = new VolatileResourceStmtContext(_ctx, getState());
		enterRule(_localctx, 70, RULE_volatileResourceStmt);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(555);
			((VolatileResourceStmtContext)_localctx).resType = match(VolatileResourceType);
			setState(556);
			((VolatileResourceStmtContext)_localctx).resName = declIdentifier();
			setState(557);
			match(ASSIGN);
			setState(558);
			((VolatileResourceStmtContext)_localctx).resVal = match(STRING);
			setState(559);
			match(SEMICOLON);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class ResourceValueContext extends ParserRuleContext {
		public KeywordsContext kws;
		public Token resVal;
		public KeywordsContext keywords() {
			return getRuleContext(KeywordsContext.class,0);
		}
		public TerminalNode CreateUUIDResource() { return getToken(JetRuleParser.CreateUUIDResource, 0); }
		public TerminalNode STRING() { return getToken(JetRuleParser.STRING, 0); }
		public ResourceValueContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_resourceValue; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterResourceValue(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitResourceValue(this);
		}
	}

	public final ResourceValueContext resourceValue() throws RecognitionException {
		ResourceValueContext _localctx = new ResourceValueContext(_ctx, getState());
		enterRule(_localctx, 72, RULE_resourceValue);
		try {
			setState(564);
			_errHandler.sync(this);
			switch (_input.LA(1)) {
			case TRUE:
			case FALSE:
			case NULL:
				enterOuterAlt(_localctx, 1);
				{
				setState(561);
				((ResourceValueContext)_localctx).kws = keywords();
				}
				break;
			case CreateUUIDResource:
				enterOuterAlt(_localctx, 2);
				{
				setState(562);
				((ResourceValueContext)_localctx).resVal = match(CreateUUIDResource);
				}
				break;
			case STRING:
				enterOuterAlt(_localctx, 3);
				{
				setState(563);
				((ResourceValueContext)_localctx).resVal = match(STRING);
				}
				break;
			default:
				throw new NoViableAltException(this);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class LookupTableStmtContext extends ParserRuleContext {
		public DeclIdentifierContext lookupName;
		public StringListContext tblKeys;
		public TerminalNode LookupTable() { return getToken(JetRuleParser.LookupTable, 0); }
		public CsvLocationContext csvLocation() {
			return getRuleContext(CsvLocationContext.class,0);
		}
		public TerminalNode Key() { return getToken(JetRuleParser.Key, 0); }
		public List<TerminalNode> ASSIGN() { return getTokens(JetRuleParser.ASSIGN); }
		public TerminalNode ASSIGN(int i) {
			return getToken(JetRuleParser.ASSIGN, i);
		}
		public TerminalNode Columns() { return getToken(JetRuleParser.Columns, 0); }
		public ColumnDefSeqContext columnDefSeq() {
			return getRuleContext(ColumnDefSeqContext.class,0);
		}
		public TerminalNode SEMICOLON() { return getToken(JetRuleParser.SEMICOLON, 0); }
		public DeclIdentifierContext declIdentifier() {
			return getRuleContext(DeclIdentifierContext.class,0);
		}
		public StringListContext stringList() {
			return getRuleContext(StringListContext.class,0);
		}
		public List<TerminalNode> COMMENT() { return getTokens(JetRuleParser.COMMENT); }
		public TerminalNode COMMENT(int i) {
			return getToken(JetRuleParser.COMMENT, i);
		}
		public LookupTableStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_lookupTableStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterLookupTableStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitLookupTableStmt(this);
		}
	}

	public final LookupTableStmtContext lookupTableStmt() throws RecognitionException {
		LookupTableStmtContext _localctx = new LookupTableStmtContext(_ctx, getState());
		enterRule(_localctx, 74, RULE_lookupTableStmt);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(566);
			match(LookupTable);
			setState(567);
			((LookupTableStmtContext)_localctx).lookupName = declIdentifier();
			setState(568);
			match(T__0);
			setState(572);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==COMMENT) {
				{
				{
				setState(569);
				match(COMMENT);
				}
				}
				setState(574);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			setState(575);
			csvLocation();
			setState(579);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==COMMENT) {
				{
				{
				setState(576);
				match(COMMENT);
				}
				}
				setState(581);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			setState(582);
			match(Key);
			setState(583);
			match(ASSIGN);
			setState(584);
			((LookupTableStmtContext)_localctx).tblKeys = stringList();
			setState(585);
			match(T__2);
			setState(589);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==COMMENT) {
				{
				{
				setState(586);
				match(COMMENT);
				}
				}
				setState(591);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			setState(592);
			match(Columns);
			setState(593);
			match(ASSIGN);
			setState(594);
			match(T__3);
			setState(598);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==COMMENT) {
				{
				{
				setState(595);
				match(COMMENT);
				}
				}
				setState(600);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			setState(601);
			columnDefSeq();
			setState(605);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==COMMENT) {
				{
				{
				setState(602);
				match(COMMENT);
				}
				}
				setState(607);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			setState(608);
			match(T__4);
			setState(610);
			_errHandler.sync(this);
			_la = _input.LA(1);
			if (_la==T__2) {
				{
				setState(609);
				match(T__2);
				}
			}

			setState(615);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==COMMENT) {
				{
				{
				setState(612);
				match(COMMENT);
				}
				}
				setState(617);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			setState(618);
			match(T__1);
			setState(619);
			match(SEMICOLON);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class CsvLocationContext extends ParserRuleContext {
		public DeclIdentifierContext tblStorageName;
		public Token csvFileName;
		public TerminalNode TableName() { return getToken(JetRuleParser.TableName, 0); }
		public TerminalNode ASSIGN() { return getToken(JetRuleParser.ASSIGN, 0); }
		public DeclIdentifierContext declIdentifier() {
			return getRuleContext(DeclIdentifierContext.class,0);
		}
		public TerminalNode CSVFileName() { return getToken(JetRuleParser.CSVFileName, 0); }
		public TerminalNode STRING() { return getToken(JetRuleParser.STRING, 0); }
		public CsvLocationContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_csvLocation; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterCsvLocation(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitCsvLocation(this);
		}
	}

	public final CsvLocationContext csvLocation() throws RecognitionException {
		CsvLocationContext _localctx = new CsvLocationContext(_ctx, getState());
		enterRule(_localctx, 76, RULE_csvLocation);
		try {
			setState(630);
			_errHandler.sync(this);
			switch (_input.LA(1)) {
			case TableName:
				enterOuterAlt(_localctx, 1);
				{
				setState(621);
				match(TableName);
				setState(622);
				match(ASSIGN);
				setState(623);
				((CsvLocationContext)_localctx).tblStorageName = declIdentifier();
				setState(624);
				match(T__2);
				}
				break;
			case CSVFileName:
				enterOuterAlt(_localctx, 2);
				{
				setState(626);
				match(CSVFileName);
				setState(627);
				match(ASSIGN);
				setState(628);
				((CsvLocationContext)_localctx).csvFileName = match(STRING);
				setState(629);
				match(T__2);
				}
				break;
			default:
				throw new NoViableAltException(this);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class StringListContext extends ParserRuleContext {
		public StringSeqContext seqCtx;
		public StringSeqContext stringSeq() {
			return getRuleContext(StringSeqContext.class,0);
		}
		public StringListContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_stringList; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterStringList(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitStringList(this);
		}
	}

	public final StringListContext stringList() throws RecognitionException {
		StringListContext _localctx = new StringListContext(_ctx, getState());
		enterRule(_localctx, 78, RULE_stringList);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(632);
			match(T__3);
			setState(634);
			_errHandler.sync(this);
			_la = _input.LA(1);
			if (_la==STRING) {
				{
				setState(633);
				((StringListContext)_localctx).seqCtx = stringSeq();
				}
			}

			setState(636);
			match(T__4);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class StringSeqContext extends ParserRuleContext {
		public Token STRING;
		public List<Token> slist = new ArrayList<Token>();
		public List<TerminalNode> STRING() { return getTokens(JetRuleParser.STRING); }
		public TerminalNode STRING(int i) {
			return getToken(JetRuleParser.STRING, i);
		}
		public StringSeqContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_stringSeq; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterStringSeq(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitStringSeq(this);
		}
	}

	public final StringSeqContext stringSeq() throws RecognitionException {
		StringSeqContext _localctx = new StringSeqContext(_ctx, getState());
		enterRule(_localctx, 80, RULE_stringSeq);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(638);
			((StringSeqContext)_localctx).STRING = match(STRING);
			((StringSeqContext)_localctx).slist.add(((StringSeqContext)_localctx).STRING);
			setState(643);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==T__2) {
				{
				{
				setState(639);
				match(T__2);
				setState(640);
				((StringSeqContext)_localctx).STRING = match(STRING);
				((StringSeqContext)_localctx).slist.add(((StringSeqContext)_localctx).STRING);
				}
				}
				setState(645);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class ColumnDefSeqContext extends ParserRuleContext {
		public List<ColumnDefinitionsContext> columnDefinitions() {
			return getRuleContexts(ColumnDefinitionsContext.class);
		}
		public ColumnDefinitionsContext columnDefinitions(int i) {
			return getRuleContext(ColumnDefinitionsContext.class,i);
		}
		public List<TerminalNode> COMMENT() { return getTokens(JetRuleParser.COMMENT); }
		public TerminalNode COMMENT(int i) {
			return getToken(JetRuleParser.COMMENT, i);
		}
		public ColumnDefSeqContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_columnDefSeq; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterColumnDefSeq(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitColumnDefSeq(this);
		}
	}

	public final ColumnDefSeqContext columnDefSeq() throws RecognitionException {
		ColumnDefSeqContext _localctx = new ColumnDefSeqContext(_ctx, getState());
		enterRule(_localctx, 82, RULE_columnDefSeq);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(646);
			columnDefinitions();
			setState(657);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==T__2) {
				{
				{
				setState(647);
				match(T__2);
				setState(651);
				_errHandler.sync(this);
				_la = _input.LA(1);
				while (_la==COMMENT) {
					{
					{
					setState(648);
					match(COMMENT);
					}
					}
					setState(653);
					_errHandler.sync(this);
					_la = _input.LA(1);
				}
				setState(654);
				columnDefinitions();
				}
				}
				setState(659);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class ColumnDefinitionsContext extends ParserRuleContext {
		public Token columnName;
		public Token array;
		public DataPropertyTypeContext columnType;
		public TerminalNode STRING() { return getToken(JetRuleParser.STRING, 0); }
		public DataPropertyTypeContext dataPropertyType() {
			return getRuleContext(DataPropertyTypeContext.class,0);
		}
		public TerminalNode ARRAY() { return getToken(JetRuleParser.ARRAY, 0); }
		public ColumnDefinitionsContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_columnDefinitions; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterColumnDefinitions(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitColumnDefinitions(this);
		}
	}

	public final ColumnDefinitionsContext columnDefinitions() throws RecognitionException {
		ColumnDefinitionsContext _localctx = new ColumnDefinitionsContext(_ctx, getState());
		enterRule(_localctx, 84, RULE_columnDefinitions);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(660);
			((ColumnDefinitionsContext)_localctx).columnName = match(STRING);
			setState(661);
			match(T__5);
			setState(663);
			_errHandler.sync(this);
			_la = _input.LA(1);
			if (_la==ARRAY) {
				{
				setState(662);
				((ColumnDefinitionsContext)_localctx).array = match(ARRAY);
				}
			}

			setState(665);
			((ColumnDefinitionsContext)_localctx).columnType = dataPropertyType();
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class JetRuleStmtContext extends ParserRuleContext {
		public Token ruleName;
		public TerminalNode SEMICOLON() { return getToken(JetRuleParser.SEMICOLON, 0); }
		public TerminalNode Identifier() { return getToken(JetRuleParser.Identifier, 0); }
		public List<RulePropertiesContext> ruleProperties() {
			return getRuleContexts(RulePropertiesContext.class);
		}
		public RulePropertiesContext ruleProperties(int i) {
			return getRuleContext(RulePropertiesContext.class,i);
		}
		public List<TerminalNode> COMMENT() { return getTokens(JetRuleParser.COMMENT); }
		public TerminalNode COMMENT(int i) {
			return getToken(JetRuleParser.COMMENT, i);
		}
		public List<AntecedentContext> antecedent() {
			return getRuleContexts(AntecedentContext.class);
		}
		public AntecedentContext antecedent(int i) {
			return getRuleContext(AntecedentContext.class,i);
		}
		public List<ConsequentContext> consequent() {
			return getRuleContexts(ConsequentContext.class);
		}
		public ConsequentContext consequent(int i) {
			return getRuleContext(ConsequentContext.class,i);
		}
		public JetRuleStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_jetRuleStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterJetRuleStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitJetRuleStmt(this);
		}
	}

	public final JetRuleStmtContext jetRuleStmt() throws RecognitionException {
		JetRuleStmtContext _localctx = new JetRuleStmtContext(_ctx, getState());
		enterRule(_localctx, 86, RULE_jetRuleStmt);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(667);
			match(T__3);
			setState(668);
			((JetRuleStmtContext)_localctx).ruleName = match(Identifier);
			setState(672);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==T__2) {
				{
				{
				setState(669);
				ruleProperties();
				}
				}
				setState(674);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			setState(675);
			match(T__4);
			setState(676);
			match(T__7);
			setState(680);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==COMMENT) {
				{
				{
				setState(677);
				match(COMMENT);
				}
				}
				setState(682);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			setState(690); 
			_errHandler.sync(this);
			_la = _input.LA(1);
			do {
				{
				{
				setState(683);
				antecedent();
				setState(687);
				_errHandler.sync(this);
				_la = _input.LA(1);
				while (_la==COMMENT) {
					{
					{
					setState(684);
					match(COMMENT);
					}
					}
					setState(689);
					_errHandler.sync(this);
					_la = _input.LA(1);
				}
				}
				}
				setState(692); 
				_errHandler.sync(this);
				_la = _input.LA(1);
			} while ( _la==T__9 || _la==NOT );
			setState(694);
			match(T__8);
			setState(698);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==COMMENT) {
				{
				{
				setState(695);
				match(COMMENT);
				}
				}
				setState(700);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			setState(708); 
			_errHandler.sync(this);
			_la = _input.LA(1);
			do {
				{
				{
				setState(701);
				consequent();
				setState(705);
				_errHandler.sync(this);
				_la = _input.LA(1);
				while (_la==COMMENT) {
					{
					{
					setState(702);
					match(COMMENT);
					}
					}
					setState(707);
					_errHandler.sync(this);
					_la = _input.LA(1);
				}
				}
				}
				setState(710); 
				_errHandler.sync(this);
				_la = _input.LA(1);
			} while ( _la==T__9 );
			setState(712);
			match(SEMICOLON);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class RulePropertiesContext extends ParserRuleContext {
		public Token key;
		public PropertyValueContext valCtx;
		public TerminalNode ASSIGN() { return getToken(JetRuleParser.ASSIGN, 0); }
		public TerminalNode Identifier() { return getToken(JetRuleParser.Identifier, 0); }
		public PropertyValueContext propertyValue() {
			return getRuleContext(PropertyValueContext.class,0);
		}
		public RulePropertiesContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_ruleProperties; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterRuleProperties(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitRuleProperties(this);
		}
	}

	public final RulePropertiesContext ruleProperties() throws RecognitionException {
		RulePropertiesContext _localctx = new RulePropertiesContext(_ctx, getState());
		enterRule(_localctx, 88, RULE_ruleProperties);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(714);
			match(T__2);
			setState(715);
			((RulePropertiesContext)_localctx).key = match(Identifier);
			setState(716);
			match(ASSIGN);
			setState(717);
			((RulePropertiesContext)_localctx).valCtx = propertyValue();
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class PropertyValueContext extends ParserRuleContext {
		public Token val;
		public IntExprContext intval;
		public TerminalNode STRING() { return getToken(JetRuleParser.STRING, 0); }
		public TerminalNode TRUE() { return getToken(JetRuleParser.TRUE, 0); }
		public TerminalNode FALSE() { return getToken(JetRuleParser.FALSE, 0); }
		public IntExprContext intExpr() {
			return getRuleContext(IntExprContext.class,0);
		}
		public PropertyValueContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_propertyValue; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterPropertyValue(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitPropertyValue(this);
		}
	}

	public final PropertyValueContext propertyValue() throws RecognitionException {
		PropertyValueContext _localctx = new PropertyValueContext(_ctx, getState());
		enterRule(_localctx, 90, RULE_propertyValue);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(723);
			_errHandler.sync(this);
			switch (_input.LA(1)) {
			case STRING:
				{
				setState(719);
				((PropertyValueContext)_localctx).val = match(STRING);
				}
				break;
			case TRUE:
				{
				setState(720);
				((PropertyValueContext)_localctx).val = match(TRUE);
				}
				break;
			case FALSE:
				{
				setState(721);
				((PropertyValueContext)_localctx).val = match(FALSE);
				}
				break;
			case PLUS:
			case MINUS:
			case DIGITS:
				{
				setState(722);
				((PropertyValueContext)_localctx).intval = intExpr();
				}
				break;
			default:
				throw new NoViableAltException(this);
			}
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class AntecedentContext extends ParserRuleContext {
		public Token n;
		public AtomContext s;
		public AtomContext p;
		public ObjectAtomContext o;
		public ExprTermContext f;
		public List<AtomContext> atom() {
			return getRuleContexts(AtomContext.class);
		}
		public AtomContext atom(int i) {
			return getRuleContext(AtomContext.class,i);
		}
		public ObjectAtomContext objectAtom() {
			return getRuleContext(ObjectAtomContext.class,0);
		}
		public TerminalNode NOT() { return getToken(JetRuleParser.NOT, 0); }
		public ExprTermContext exprTerm() {
			return getRuleContext(ExprTermContext.class,0);
		}
		public AntecedentContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_antecedent; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterAntecedent(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitAntecedent(this);
		}
	}

	public final AntecedentContext antecedent() throws RecognitionException {
		AntecedentContext _localctx = new AntecedentContext(_ctx, getState());
		enterRule(_localctx, 92, RULE_antecedent);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(726);
			_errHandler.sync(this);
			_la = _input.LA(1);
			if (_la==NOT) {
				{
				setState(725);
				((AntecedentContext)_localctx).n = match(NOT);
				}
			}

			setState(728);
			match(T__9);
			setState(729);
			((AntecedentContext)_localctx).s = atom();
			setState(730);
			((AntecedentContext)_localctx).p = atom();
			setState(731);
			((AntecedentContext)_localctx).o = objectAtom();
			setState(732);
			match(T__10);
			setState(734);
			_errHandler.sync(this);
			_la = _input.LA(1);
			if (_la==T__6) {
				{
				setState(733);
				match(T__6);
				}
			}

			setState(742);
			_errHandler.sync(this);
			_la = _input.LA(1);
			if (_la==T__3) {
				{
				setState(736);
				match(T__3);
				setState(737);
				((AntecedentContext)_localctx).f = exprTerm(0);
				setState(738);
				match(T__4);
				setState(740);
				_errHandler.sync(this);
				_la = _input.LA(1);
				if (_la==T__6) {
					{
					setState(739);
					match(T__6);
					}
				}

				}
			}

			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class ConsequentContext extends ParserRuleContext {
		public AtomContext s;
		public AtomContext p;
		public ExprTermContext o;
		public List<AtomContext> atom() {
			return getRuleContexts(AtomContext.class);
		}
		public AtomContext atom(int i) {
			return getRuleContext(AtomContext.class,i);
		}
		public ExprTermContext exprTerm() {
			return getRuleContext(ExprTermContext.class,0);
		}
		public ConsequentContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_consequent; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterConsequent(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitConsequent(this);
		}
	}

	public final ConsequentContext consequent() throws RecognitionException {
		ConsequentContext _localctx = new ConsequentContext(_ctx, getState());
		enterRule(_localctx, 94, RULE_consequent);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(744);
			match(T__9);
			setState(745);
			((ConsequentContext)_localctx).s = atom();
			setState(746);
			((ConsequentContext)_localctx).p = atom();
			setState(747);
			((ConsequentContext)_localctx).o = exprTerm(0);
			setState(748);
			match(T__10);
			setState(750);
			_errHandler.sync(this);
			_la = _input.LA(1);
			if (_la==T__6) {
				{
				setState(749);
				match(T__6);
				}
			}

			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class AtomContext extends ParserRuleContext {
		public TerminalNode Identifier() { return getToken(JetRuleParser.Identifier, 0); }
		public DeclIdentifierContext declIdentifier() {
			return getRuleContext(DeclIdentifierContext.class,0);
		}
		public AtomContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_atom; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterAtom(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitAtom(this);
		}
	}

	public final AtomContext atom() throws RecognitionException {
		AtomContext _localctx = new AtomContext(_ctx, getState());
		enterRule(_localctx, 96, RULE_atom);
		try {
			setState(755);
			_errHandler.sync(this);
			switch (_input.LA(1)) {
			case T__11:
				enterOuterAlt(_localctx, 1);
				{
				setState(752);
				match(T__11);
				setState(753);
				match(Identifier);
				}
				break;
			case Identifier:
				enterOuterAlt(_localctx, 2);
				{
				setState(754);
				declIdentifier();
				}
				break;
			default:
				throw new NoViableAltException(this);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class ObjectAtomContext extends ParserRuleContext {
		public KeywordsContext kws;
		public AtomContext atom() {
			return getRuleContext(AtomContext.class,0);
		}
		public TerminalNode Int32Type() { return getToken(JetRuleParser.Int32Type, 0); }
		public IntExprContext intExpr() {
			return getRuleContext(IntExprContext.class,0);
		}
		public TerminalNode UInt32Type() { return getToken(JetRuleParser.UInt32Type, 0); }
		public UintExprContext uintExpr() {
			return getRuleContext(UintExprContext.class,0);
		}
		public TerminalNode Int64Type() { return getToken(JetRuleParser.Int64Type, 0); }
		public TerminalNode UInt64Type() { return getToken(JetRuleParser.UInt64Type, 0); }
		public TerminalNode DoubleType() { return getToken(JetRuleParser.DoubleType, 0); }
		public DoubleExprContext doubleExpr() {
			return getRuleContext(DoubleExprContext.class,0);
		}
		public TerminalNode StringType() { return getToken(JetRuleParser.StringType, 0); }
		public TerminalNode STRING() { return getToken(JetRuleParser.STRING, 0); }
		public TerminalNode DateType() { return getToken(JetRuleParser.DateType, 0); }
		public TerminalNode DatetimeType() { return getToken(JetRuleParser.DatetimeType, 0); }
		public TerminalNode BoolType() { return getToken(JetRuleParser.BoolType, 0); }
		public KeywordsContext keywords() {
			return getRuleContext(KeywordsContext.class,0);
		}
		public ObjectAtomContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_objectAtom; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterObjectAtom(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitObjectAtom(this);
		}
	}

	public final ObjectAtomContext objectAtom() throws RecognitionException {
		ObjectAtomContext _localctx = new ObjectAtomContext(_ctx, getState());
		enterRule(_localctx, 98, RULE_objectAtom);
		try {
			setState(802);
			_errHandler.sync(this);
			switch (_input.LA(1)) {
			case T__11:
			case Identifier:
				enterOuterAlt(_localctx, 1);
				{
				setState(757);
				atom();
				}
				break;
			case Int32Type:
				enterOuterAlt(_localctx, 2);
				{
				setState(758);
				match(Int32Type);
				setState(759);
				match(T__9);
				setState(760);
				intExpr();
				setState(761);
				match(T__10);
				}
				break;
			case UInt32Type:
				enterOuterAlt(_localctx, 3);
				{
				setState(763);
				match(UInt32Type);
				setState(764);
				match(T__9);
				setState(765);
				uintExpr();
				setState(766);
				match(T__10);
				}
				break;
			case Int64Type:
				enterOuterAlt(_localctx, 4);
				{
				setState(768);
				match(Int64Type);
				setState(769);
				match(T__9);
				setState(770);
				intExpr();
				setState(771);
				match(T__10);
				}
				break;
			case UInt64Type:
				enterOuterAlt(_localctx, 5);
				{
				setState(773);
				match(UInt64Type);
				setState(774);
				match(T__9);
				setState(775);
				uintExpr();
				setState(776);
				match(T__10);
				}
				break;
			case DoubleType:
				enterOuterAlt(_localctx, 6);
				{
				setState(778);
				match(DoubleType);
				setState(779);
				match(T__9);
				setState(780);
				doubleExpr();
				setState(781);
				match(T__10);
				}
				break;
			case StringType:
				enterOuterAlt(_localctx, 7);
				{
				setState(783);
				match(StringType);
				setState(784);
				match(T__9);
				setState(785);
				match(STRING);
				setState(786);
				match(T__10);
				}
				break;
			case DateType:
				enterOuterAlt(_localctx, 8);
				{
				setState(787);
				match(DateType);
				setState(788);
				match(T__9);
				setState(789);
				match(STRING);
				setState(790);
				match(T__10);
				}
				break;
			case DatetimeType:
				enterOuterAlt(_localctx, 9);
				{
				setState(791);
				match(DatetimeType);
				setState(792);
				match(T__9);
				setState(793);
				match(STRING);
				setState(794);
				match(T__10);
				}
				break;
			case BoolType:
				enterOuterAlt(_localctx, 10);
				{
				setState(795);
				match(BoolType);
				setState(796);
				match(T__9);
				setState(797);
				match(STRING);
				setState(798);
				match(T__10);
				}
				break;
			case STRING:
				enterOuterAlt(_localctx, 11);
				{
				setState(799);
				match(STRING);
				}
				break;
			case TRUE:
			case FALSE:
			case NULL:
				enterOuterAlt(_localctx, 12);
				{
				setState(800);
				((ObjectAtomContext)_localctx).kws = keywords();
				}
				break;
			case PLUS:
			case MINUS:
			case DIGITS:
				enterOuterAlt(_localctx, 13);
				{
				setState(801);
				doubleExpr();
				}
				break;
			default:
				throw new NoViableAltException(this);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class KeywordsContext extends ParserRuleContext {
		public TerminalNode TRUE() { return getToken(JetRuleParser.TRUE, 0); }
		public TerminalNode FALSE() { return getToken(JetRuleParser.FALSE, 0); }
		public TerminalNode NULL() { return getToken(JetRuleParser.NULL, 0); }
		public KeywordsContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_keywords; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterKeywords(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitKeywords(this);
		}
	}

	public final KeywordsContext keywords() throws RecognitionException {
		KeywordsContext _localctx = new KeywordsContext(_ctx, getState());
		enterRule(_localctx, 100, RULE_keywords);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(804);
			_la = _input.LA(1);
			if ( !((((_la) & ~0x3f) == 0 && ((1L << _la) & 492581209243648L) != 0)) ) {
			_errHandler.recoverInline(this);
			}
			else {
				if ( _input.LA(1)==Token.EOF ) matchedEOF = true;
				_errHandler.reportMatch(this);
				consume();
			}
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class ExprTermContext extends ParserRuleContext {
		public ExprTermContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_exprTerm; }
	 
		public ExprTermContext() { }
		public void copyFrom(ExprTermContext ctx) {
			super.copyFrom(ctx);
		}
	}
	@SuppressWarnings("CheckReturnValue")
	public static class SelfExprTermContext extends ExprTermContext {
		public ExprTermContext selfExpr;
		public ExprTermContext exprTerm() {
			return getRuleContext(ExprTermContext.class,0);
		}
		public SelfExprTermContext(ExprTermContext ctx) { copyFrom(ctx); }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterSelfExprTerm(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitSelfExprTerm(this);
		}
	}
	@SuppressWarnings("CheckReturnValue")
	public static class BinaryExprTerm2Context extends ExprTermContext {
		public ExprTermContext lhs;
		public BinaryOpContext op;
		public ExprTermContext rhs;
		public List<ExprTermContext> exprTerm() {
			return getRuleContexts(ExprTermContext.class);
		}
		public ExprTermContext exprTerm(int i) {
			return getRuleContext(ExprTermContext.class,i);
		}
		public BinaryOpContext binaryOp() {
			return getRuleContext(BinaryOpContext.class,0);
		}
		public BinaryExprTerm2Context(ExprTermContext ctx) { copyFrom(ctx); }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterBinaryExprTerm2(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitBinaryExprTerm2(this);
		}
	}
	@SuppressWarnings("CheckReturnValue")
	public static class UnaryExprTermContext extends ExprTermContext {
		public UnaryOpContext op;
		public ExprTermContext arg;
		public UnaryOpContext unaryOp() {
			return getRuleContext(UnaryOpContext.class,0);
		}
		public ExprTermContext exprTerm() {
			return getRuleContext(ExprTermContext.class,0);
		}
		public UnaryExprTermContext(ExprTermContext ctx) { copyFrom(ctx); }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterUnaryExprTerm(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitUnaryExprTerm(this);
		}
	}
	@SuppressWarnings("CheckReturnValue")
	public static class ObjectAtomExprTermContext extends ExprTermContext {
		public ObjectAtomContext ident;
		public ObjectAtomContext objectAtom() {
			return getRuleContext(ObjectAtomContext.class,0);
		}
		public ObjectAtomExprTermContext(ExprTermContext ctx) { copyFrom(ctx); }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterObjectAtomExprTerm(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitObjectAtomExprTerm(this);
		}
	}
	@SuppressWarnings("CheckReturnValue")
	public static class UnaryExprTerm3Context extends ExprTermContext {
		public UnaryOpContext op;
		public ExprTermContext arg;
		public UnaryOpContext unaryOp() {
			return getRuleContext(UnaryOpContext.class,0);
		}
		public ExprTermContext exprTerm() {
			return getRuleContext(ExprTermContext.class,0);
		}
		public UnaryExprTerm3Context(ExprTermContext ctx) { copyFrom(ctx); }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterUnaryExprTerm3(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitUnaryExprTerm3(this);
		}
	}
	@SuppressWarnings("CheckReturnValue")
	public static class UnaryExprTerm2Context extends ExprTermContext {
		public UnaryOpContext op;
		public ExprTermContext arg;
		public UnaryOpContext unaryOp() {
			return getRuleContext(UnaryOpContext.class,0);
		}
		public ExprTermContext exprTerm() {
			return getRuleContext(ExprTermContext.class,0);
		}
		public UnaryExprTerm2Context(ExprTermContext ctx) { copyFrom(ctx); }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterUnaryExprTerm2(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitUnaryExprTerm2(this);
		}
	}
	@SuppressWarnings("CheckReturnValue")
	public static class BinaryExprTermContext extends ExprTermContext {
		public ExprTermContext lhs;
		public BinaryOpContext op;
		public ExprTermContext rhs;
		public List<ExprTermContext> exprTerm() {
			return getRuleContexts(ExprTermContext.class);
		}
		public ExprTermContext exprTerm(int i) {
			return getRuleContext(ExprTermContext.class,i);
		}
		public BinaryOpContext binaryOp() {
			return getRuleContext(BinaryOpContext.class,0);
		}
		public BinaryExprTermContext(ExprTermContext ctx) { copyFrom(ctx); }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterBinaryExprTerm(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitBinaryExprTerm(this);
		}
	}

	public final ExprTermContext exprTerm() throws RecognitionException {
		return exprTerm(0);
	}

	private ExprTermContext exprTerm(int _p) throws RecognitionException {
		ParserRuleContext _parentctx = _ctx;
		int _parentState = getState();
		ExprTermContext _localctx = new ExprTermContext(_ctx, _parentState);
		ExprTermContext _prevctx = _localctx;
		int _startState = 102;
		enterRecursionRule(_localctx, 102, RULE_exprTerm, _p);
		try {
			int _alt;
			enterOuterAlt(_localctx, 1);
			{
			setState(831);
			_errHandler.sync(this);
			switch ( getInterpreter().adaptivePredict(_input,77,_ctx) ) {
			case 1:
				{
				_localctx = new BinaryExprTerm2Context(_localctx);
				_ctx = _localctx;
				_prevctx = _localctx;

				setState(807);
				match(T__9);
				setState(808);
				((BinaryExprTerm2Context)_localctx).lhs = exprTerm(0);
				setState(809);
				((BinaryExprTerm2Context)_localctx).op = binaryOp();
				setState(810);
				((BinaryExprTerm2Context)_localctx).rhs = exprTerm(0);
				setState(811);
				match(T__10);
				}
				break;
			case 2:
				{
				_localctx = new UnaryExprTermContext(_localctx);
				_ctx = _localctx;
				_prevctx = _localctx;
				setState(813);
				((UnaryExprTermContext)_localctx).op = unaryOp();
				setState(814);
				match(T__9);
				setState(815);
				((UnaryExprTermContext)_localctx).arg = exprTerm(0);
				setState(816);
				match(T__10);
				}
				break;
			case 3:
				{
				_localctx = new UnaryExprTerm2Context(_localctx);
				_ctx = _localctx;
				_prevctx = _localctx;
				setState(818);
				match(T__9);
				setState(819);
				((UnaryExprTerm2Context)_localctx).op = unaryOp();
				setState(820);
				((UnaryExprTerm2Context)_localctx).arg = exprTerm(0);
				setState(821);
				match(T__10);
				}
				break;
			case 4:
				{
				_localctx = new SelfExprTermContext(_localctx);
				_ctx = _localctx;
				_prevctx = _localctx;
				setState(823);
				match(T__9);
				setState(824);
				((SelfExprTermContext)_localctx).selfExpr = exprTerm(0);
				setState(825);
				match(T__10);
				}
				break;
			case 5:
				{
				_localctx = new UnaryExprTerm3Context(_localctx);
				_ctx = _localctx;
				_prevctx = _localctx;
				setState(827);
				((UnaryExprTerm3Context)_localctx).op = unaryOp();
				setState(828);
				((UnaryExprTerm3Context)_localctx).arg = exprTerm(2);
				}
				break;
			case 6:
				{
				_localctx = new ObjectAtomExprTermContext(_localctx);
				_ctx = _localctx;
				_prevctx = _localctx;
				setState(830);
				((ObjectAtomExprTermContext)_localctx).ident = objectAtom();
				}
				break;
			}
			_ctx.stop = _input.LT(-1);
			setState(839);
			_errHandler.sync(this);
			_alt = getInterpreter().adaptivePredict(_input,78,_ctx);
			while ( _alt!=2 && _alt!=org.antlr.v4.runtime.atn.ATN.INVALID_ALT_NUMBER ) {
				if ( _alt==1 ) {
					if ( _parseListeners!=null ) triggerExitRuleEvent();
					_prevctx = _localctx;
					{
					{
					_localctx = new BinaryExprTermContext(new ExprTermContext(_parentctx, _parentState));
					((BinaryExprTermContext)_localctx).lhs = _prevctx;
					pushNewRecursionContext(_localctx, _startState, RULE_exprTerm);
					setState(833);
					if (!(precpred(_ctx, 7))) throw new FailedPredicateException(this, "precpred(_ctx, 7)");
					setState(834);
					((BinaryExprTermContext)_localctx).op = binaryOp();
					setState(835);
					((BinaryExprTermContext)_localctx).rhs = exprTerm(8);
					}
					} 
				}
				setState(841);
				_errHandler.sync(this);
				_alt = getInterpreter().adaptivePredict(_input,78,_ctx);
			}
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			unrollRecursionContexts(_parentctx);
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class BinaryOpContext extends ParserRuleContext {
		public TerminalNode PLUS() { return getToken(JetRuleParser.PLUS, 0); }
		public TerminalNode EQ() { return getToken(JetRuleParser.EQ, 0); }
		public TerminalNode LT() { return getToken(JetRuleParser.LT, 0); }
		public TerminalNode LE() { return getToken(JetRuleParser.LE, 0); }
		public TerminalNode GT() { return getToken(JetRuleParser.GT, 0); }
		public TerminalNode GE() { return getToken(JetRuleParser.GE, 0); }
		public TerminalNode NE() { return getToken(JetRuleParser.NE, 0); }
		public TerminalNode REGEX2() { return getToken(JetRuleParser.REGEX2, 0); }
		public TerminalNode MINUS() { return getToken(JetRuleParser.MINUS, 0); }
		public TerminalNode MUL() { return getToken(JetRuleParser.MUL, 0); }
		public TerminalNode DIV() { return getToken(JetRuleParser.DIV, 0); }
		public TerminalNode OR() { return getToken(JetRuleParser.OR, 0); }
		public TerminalNode AND() { return getToken(JetRuleParser.AND, 0); }
		public TerminalNode Identifier() { return getToken(JetRuleParser.Identifier, 0); }
		public BinaryOpContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_binaryOp; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterBinaryOp(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitBinaryOp(this);
		}
	}

	public final BinaryOpContext binaryOp() throws RecognitionException {
		BinaryOpContext _localctx = new BinaryOpContext(_ctx, getState());
		enterRule(_localctx, 104, RULE_binaryOp);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(842);
			_la = _input.LA(1);
			if ( !(((((_la - 51)) & ~0x3f) == 0 && ((1L << (_la - 51)) & 40959L) != 0)) ) {
			_errHandler.recoverInline(this);
			}
			else {
				if ( _input.LA(1)==Token.EOF ) matchedEOF = true;
				_errHandler.reportMatch(this);
				consume();
			}
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class UnaryOpContext extends ParserRuleContext {
		public TerminalNode NOT() { return getToken(JetRuleParser.NOT, 0); }
		public TerminalNode TOTEXT() { return getToken(JetRuleParser.TOTEXT, 0); }
		public TerminalNode Identifier() { return getToken(JetRuleParser.Identifier, 0); }
		public UnaryOpContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_unaryOp; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterUnaryOp(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitUnaryOp(this);
		}
	}

	public final UnaryOpContext unaryOp() throws RecognitionException {
		UnaryOpContext _localctx = new UnaryOpContext(_ctx, getState());
		enterRule(_localctx, 106, RULE_unaryOp);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(844);
			_la = _input.LA(1);
			if ( !(((((_la - 49)) & ~0x3f) == 0 && ((1L << (_la - 49)) & 131075L) != 0)) ) {
			_errHandler.recoverInline(this);
			}
			else {
				if ( _input.LA(1)==Token.EOF ) matchedEOF = true;
				_errHandler.reportMatch(this);
				consume();
			}
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	@SuppressWarnings("CheckReturnValue")
	public static class TripleStmtContext extends ParserRuleContext {
		public AtomContext s;
		public AtomContext p;
		public ObjectAtomContext o;
		public TerminalNode TRIPLE() { return getToken(JetRuleParser.TRIPLE, 0); }
		public TerminalNode SEMICOLON() { return getToken(JetRuleParser.SEMICOLON, 0); }
		public List<AtomContext> atom() {
			return getRuleContexts(AtomContext.class);
		}
		public AtomContext atom(int i) {
			return getRuleContext(AtomContext.class,i);
		}
		public ObjectAtomContext objectAtom() {
			return getRuleContext(ObjectAtomContext.class,0);
		}
		public TripleStmtContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_tripleStmt; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).enterTripleStmt(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof JetRuleListener ) ((JetRuleListener)listener).exitTripleStmt(this);
		}
	}

	public final TripleStmtContext tripleStmt() throws RecognitionException {
		TripleStmtContext _localctx = new TripleStmtContext(_ctx, getState());
		enterRule(_localctx, 108, RULE_tripleStmt);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(846);
			match(TRIPLE);
			setState(847);
			match(T__9);
			setState(848);
			((TripleStmtContext)_localctx).s = atom();
			setState(849);
			match(T__2);
			setState(850);
			((TripleStmtContext)_localctx).p = atom();
			setState(851);
			match(T__2);
			setState(852);
			((TripleStmtContext)_localctx).o = objectAtom();
			setState(853);
			match(T__10);
			setState(854);
			match(SEMICOLON);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	public boolean sempred(RuleContext _localctx, int ruleIndex, int predIndex) {
		switch (ruleIndex) {
		case 51:
			return exprTerm_sempred((ExprTermContext)_localctx, predIndex);
		}
		return true;
	}
	private boolean exprTerm_sempred(ExprTermContext _localctx, int predIndex) {
		switch (predIndex) {
		case 0:
			return precpred(_ctx, 7);
		}
		return true;
	}

	public static final String _serializedATN =
		"\u0004\u0001F\u0359\u0002\u0000\u0007\u0000\u0002\u0001\u0007\u0001\u0002"+
		"\u0002\u0007\u0002\u0002\u0003\u0007\u0003\u0002\u0004\u0007\u0004\u0002"+
		"\u0005\u0007\u0005\u0002\u0006\u0007\u0006\u0002\u0007\u0007\u0007\u0002"+
		"\b\u0007\b\u0002\t\u0007\t\u0002\n\u0007\n\u0002\u000b\u0007\u000b\u0002"+
		"\f\u0007\f\u0002\r\u0007\r\u0002\u000e\u0007\u000e\u0002\u000f\u0007\u000f"+
		"\u0002\u0010\u0007\u0010\u0002\u0011\u0007\u0011\u0002\u0012\u0007\u0012"+
		"\u0002\u0013\u0007\u0013\u0002\u0014\u0007\u0014\u0002\u0015\u0007\u0015"+
		"\u0002\u0016\u0007\u0016\u0002\u0017\u0007\u0017\u0002\u0018\u0007\u0018"+
		"\u0002\u0019\u0007\u0019\u0002\u001a\u0007\u001a\u0002\u001b\u0007\u001b"+
		"\u0002\u001c\u0007\u001c\u0002\u001d\u0007\u001d\u0002\u001e\u0007\u001e"+
		"\u0002\u001f\u0007\u001f\u0002 \u0007 \u0002!\u0007!\u0002\"\u0007\"\u0002"+
		"#\u0007#\u0002$\u0007$\u0002%\u0007%\u0002&\u0007&\u0002\'\u0007\'\u0002"+
		"(\u0007(\u0002)\u0007)\u0002*\u0007*\u0002+\u0007+\u0002,\u0007,\u0002"+
		"-\u0007-\u0002.\u0007.\u0002/\u0007/\u00020\u00070\u00021\u00071\u0002"+
		"2\u00072\u00023\u00073\u00024\u00074\u00025\u00075\u00026\u00076\u0001"+
		"\u0000\u0005\u0000p\b\u0000\n\u0000\f\u0000s\t\u0000\u0001\u0000\u0001"+
		"\u0000\u0001\u0001\u0001\u0001\u0001\u0001\u0001\u0001\u0001\u0001\u0001"+
		"\u0001\u0001\u0001\u0001\u0001\u0001\u0001\u0001\u0001\u0003\u0001\u0081"+
		"\b\u0001\u0001\u0002\u0001\u0002\u0001\u0002\u0001\u0002\u0001\u0002\u0001"+
		"\u0002\u0001\u0003\u0001\u0003\u0001\u0003\u0005\u0003\u008c\b\u0003\n"+
		"\u0003\f\u0003\u008f\t\u0003\u0001\u0003\u0001\u0003\u0005\u0003\u0093"+
		"\b\u0003\n\u0003\f\u0003\u0096\t\u0003\u0001\u0003\u0001\u0003\u0001\u0003"+
		"\u0001\u0004\u0001\u0004\u0001\u0005\u0001\u0005\u0001\u0005\u0005\u0005"+
		"\u00a0\b\u0005\n\u0005\f\u0005\u00a3\t\u0005\u0001\u0005\u0005\u0005\u00a6"+
		"\b\u0005\n\u0005\f\u0005\u00a9\t\u0005\u0001\u0006\u0001\u0006\u0001\u0006"+
		"\u0001\u0006\u0001\u0006\u0001\u0006\u0001\u0006\u0001\u0006\u0001\u0006"+
		"\u0001\u0006\u0005\u0006\u00b5\b\u0006\n\u0006\f\u0006\u00b8\t\u0006\u0001"+
		"\u0006\u0001\u0006\u0001\u0006\u0005\u0006\u00bd\b\u0006\n\u0006\f\u0006"+
		"\u00c0\t\u0006\u0001\u0006\u0005\u0006\u00c3\b\u0006\n\u0006\f\u0006\u00c6"+
		"\t\u0006\u0001\u0006\u0005\u0006\u00c9\b\u0006\n\u0006\f\u0006\u00cc\t"+
		"\u0006\u0001\u0006\u0001\u0006\u0003\u0006\u00d0\b\u0006\u0001\u0007\u0001"+
		"\u0007\u0001\u0007\u0001\u0007\u0005\u0007\u00d6\b\u0007\n\u0007\f\u0007"+
		"\u00d9\t\u0007\u0001\u0007\u0001\u0007\u0001\u0007\u0005\u0007\u00de\b"+
		"\u0007\n\u0007\f\u0007\u00e1\t\u0007\u0001\u0007\u0005\u0007\u00e4\b\u0007"+
		"\n\u0007\f\u0007\u00e7\t\u0007\u0001\u0007\u0005\u0007\u00ea\b\u0007\n"+
		"\u0007\f\u0007\u00ed\t\u0007\u0001\u0007\u0001\u0007\u0001\u0007\u0001"+
		"\b\u0001\b\u0001\b\u0001\b\u0005\b\u00f6\b\b\n\b\f\b\u00f9\t\b\u0001\b"+
		"\u0001\b\u0001\b\u0005\b\u00fe\b\b\n\b\f\b\u0101\t\b\u0001\b\u0005\b\u0104"+
		"\b\b\n\b\f\b\u0107\t\b\u0001\b\u0005\b\u010a\b\b\n\b\f\b\u010d\t\b\u0001"+
		"\b\u0001\b\u0001\b\u0001\b\u0001\b\u0001\b\u0005\b\u0115\b\b\n\b\f\b\u0118"+
		"\t\b\u0001\b\u0001\b\u0001\b\u0005\b\u011d\b\b\n\b\f\b\u0120\t\b\u0001"+
		"\b\u0005\b\u0123\b\b\n\b\f\b\u0126\t\b\u0001\b\u0005\b\u0129\b\b\n\b\f"+
		"\b\u012c\t\b\u0001\b\u0001\b\u0001\b\u0001\b\u0001\b\u0001\b\u0005\b\u0134"+
		"\b\b\n\b\f\b\u0137\t\b\u0001\b\u0001\b\u0001\b\u0005\b\u013c\b\b\n\b\f"+
		"\b\u013f\t\b\u0001\b\u0005\b\u0142\b\b\n\b\f\b\u0145\t\b\u0001\b\u0005"+
		"\b\u0148\b\b\n\b\f\b\u014b\t\b\u0001\b\u0001\b\u0001\b\u0001\b\u0001\b"+
		"\u0001\b\u0005\b\u0153\b\b\n\b\f\b\u0156\t\b\u0001\b\u0001\b\u0001\b\u0005"+
		"\b\u015b\b\b\n\b\f\b\u015e\t\b\u0001\b\u0005\b\u0161\b\b\n\b\f\b\u0164"+
		"\t\b\u0001\b\u0005\b\u0167\b\b\n\b\f\b\u016a\t\b\u0001\b\u0001\b\u0001"+
		"\b\u0003\b\u016f\b\b\u0001\t\u0001\t\u0001\n\u0001\n\u0001\n\u0003\n\u0176"+
		"\b\n\u0001\n\u0001\n\u0001\u000b\u0001\u000b\u0001\u000b\u0003\u000b\u017d"+
		"\b\u000b\u0001\u000b\u0001\u000b\u0001\f\u0001\f\u0001\r\u0001\r\u0001"+
		"\u000e\u0001\u000e\u0001\u000e\u0001\u000e\u0001\u000f\u0001\u000f\u0001"+
		"\u0010\u0001\u0010\u0001\u0010\u0001\u0010\u0005\u0010\u018f\b\u0010\n"+
		"\u0010\f\u0010\u0192\t\u0010\u0001\u0010\u0001\u0010\u0001\u0010\u0001"+
		"\u0010\u0005\u0010\u0198\b\u0010\n\u0010\f\u0010\u019b\t\u0010\u0001\u0010"+
		"\u0001\u0010\u0005\u0010\u019f\b\u0010\n\u0010\f\u0010\u01a2\t\u0010\u0001"+
		"\u0010\u0001\u0010\u0003\u0010\u01a6\b\u0010\u0001\u0010\u0005\u0010\u01a9"+
		"\b\u0010\n\u0010\f\u0010\u01ac\t\u0010\u0001\u0010\u0001\u0010\u0001\u0010"+
		"\u0001\u0011\u0001\u0011\u0001\u0011\u0005\u0011\u01b4\b\u0011\n\u0011"+
		"\f\u0011\u01b7\t\u0011\u0001\u0011\u0005\u0011\u01ba\b\u0011\n\u0011\f"+
		"\u0011\u01bd\t\u0011\u0001\u0012\u0001\u0012\u0001\u0013\u0001\u0013\u0001"+
		"\u0013\u0001\u0013\u0001\u0013\u0001\u0013\u0001\u0013\u0001\u0013\u0001"+
		"\u0013\u0003\u0013\u01ca\b\u0013\u0001\u0014\u0001\u0014\u0001\u0014\u0001"+
		"\u0014\u0001\u0014\u0001\u0014\u0001\u0015\u0001\u0015\u0001\u0015\u0001"+
		"\u0015\u0001\u0015\u0001\u0015\u0001\u0016\u0001\u0016\u0001\u0016\u0001"+
		"\u0016\u0001\u0016\u0001\u0016\u0001\u0017\u0001\u0017\u0001\u0017\u0001"+
		"\u0017\u0001\u0017\u0001\u0017\u0001\u0018\u0001\u0018\u0001\u0018\u0001"+
		"\u0018\u0001\u0018\u0001\u0018\u0001\u0019\u0001\u0019\u0001\u0019\u0001"+
		"\u0019\u0001\u0019\u0001\u0019\u0001\u001a\u0001\u001a\u0001\u001a\u0001"+
		"\u001a\u0001\u001a\u0001\u001a\u0001\u001b\u0001\u001b\u0001\u001b\u0001"+
		"\u001b\u0001\u001b\u0001\u001b\u0001\u001c\u0001\u001c\u0001\u001c\u0001"+
		"\u001c\u0001\u001c\u0001\u001c\u0001\u001d\u0001\u001d\u0001\u001d\u0001"+
		"\u001d\u0001\u001d\u0003\u001d\u0207\b\u001d\u0001\u001e\u0001\u001e\u0001"+
		"\u001e\u0003\u001e\u020c\b\u001e\u0001\u001f\u0001\u001f\u0001\u001f\u0001"+
		"\u001f\u0001\u001f\u0001\u001f\u0001\u001f\u0003\u001f\u0215\b\u001f\u0003"+
		"\u001f\u0217\b\u001f\u0001 \u0001 \u0001 \u0001 \u0001 \u0001 \u0001 "+
		"\u0003 \u0220\b \u0001!\u0001!\u0003!\u0224\b!\u0001\"\u0001\"\u0001\""+
		"\u0001\"\u0001\"\u0001\"\u0001#\u0001#\u0001#\u0001#\u0001#\u0001#\u0001"+
		"$\u0001$\u0001$\u0003$\u0235\b$\u0001%\u0001%\u0001%\u0001%\u0005%\u023b"+
		"\b%\n%\f%\u023e\t%\u0001%\u0001%\u0005%\u0242\b%\n%\f%\u0245\t%\u0001"+
		"%\u0001%\u0001%\u0001%\u0001%\u0005%\u024c\b%\n%\f%\u024f\t%\u0001%\u0001"+
		"%\u0001%\u0001%\u0005%\u0255\b%\n%\f%\u0258\t%\u0001%\u0001%\u0005%\u025c"+
		"\b%\n%\f%\u025f\t%\u0001%\u0001%\u0003%\u0263\b%\u0001%\u0005%\u0266\b"+
		"%\n%\f%\u0269\t%\u0001%\u0001%\u0001%\u0001&\u0001&\u0001&\u0001&\u0001"+
		"&\u0001&\u0001&\u0001&\u0001&\u0003&\u0277\b&\u0001\'\u0001\'\u0003\'"+
		"\u027b\b\'\u0001\'\u0001\'\u0001(\u0001(\u0001(\u0005(\u0282\b(\n(\f("+
		"\u0285\t(\u0001)\u0001)\u0001)\u0005)\u028a\b)\n)\f)\u028d\t)\u0001)\u0005"+
		")\u0290\b)\n)\f)\u0293\t)\u0001*\u0001*\u0001*\u0003*\u0298\b*\u0001*"+
		"\u0001*\u0001+\u0001+\u0001+\u0005+\u029f\b+\n+\f+\u02a2\t+\u0001+\u0001"+
		"+\u0001+\u0005+\u02a7\b+\n+\f+\u02aa\t+\u0001+\u0001+\u0005+\u02ae\b+"+
		"\n+\f+\u02b1\t+\u0004+\u02b3\b+\u000b+\f+\u02b4\u0001+\u0001+\u0005+\u02b9"+
		"\b+\n+\f+\u02bc\t+\u0001+\u0001+\u0005+\u02c0\b+\n+\f+\u02c3\t+\u0004"+
		"+\u02c5\b+\u000b+\f+\u02c6\u0001+\u0001+\u0001,\u0001,\u0001,\u0001,\u0001"+
		",\u0001-\u0001-\u0001-\u0001-\u0003-\u02d4\b-\u0001.\u0003.\u02d7\b.\u0001"+
		".\u0001.\u0001.\u0001.\u0001.\u0001.\u0003.\u02df\b.\u0001.\u0001.\u0001"+
		".\u0001.\u0003.\u02e5\b.\u0003.\u02e7\b.\u0001/\u0001/\u0001/\u0001/\u0001"+
		"/\u0001/\u0003/\u02ef\b/\u00010\u00010\u00010\u00030\u02f4\b0\u00011\u0001"+
		"1\u00011\u00011\u00011\u00011\u00011\u00011\u00011\u00011\u00011\u0001"+
		"1\u00011\u00011\u00011\u00011\u00011\u00011\u00011\u00011\u00011\u0001"+
		"1\u00011\u00011\u00011\u00011\u00011\u00011\u00011\u00011\u00011\u0001"+
		"1\u00011\u00011\u00011\u00011\u00011\u00011\u00011\u00011\u00011\u0001"+
		"1\u00011\u00011\u00011\u00031\u0323\b1\u00012\u00012\u00013\u00013\u0001"+
		"3\u00013\u00013\u00013\u00013\u00013\u00013\u00013\u00013\u00013\u0001"+
		"3\u00013\u00013\u00013\u00013\u00013\u00013\u00013\u00013\u00013\u0001"+
		"3\u00013\u00013\u00033\u0340\b3\u00013\u00013\u00013\u00013\u00053\u0346"+
		"\b3\n3\f3\u0349\t3\u00014\u00014\u00015\u00015\u00016\u00016\u00016\u0001"+
		"6\u00016\u00016\u00016\u00016\u00016\u00016\u00016\u0000\u0001f7\u0000"+
		"\u0002\u0004\u0006\b\n\f\u000e\u0010\u0012\u0014\u0016\u0018\u001a\u001c"+
		"\u001e \"$&(*,.02468:<>@BDFHJLNPRTVXZ\\^`bdfhjl\u0000\u0006\u0001\u0000"+
		"\u0015\u0016\u0001\u0000\u001d&\u0001\u0000./\u0001\u0000.0\u0002\u0000"+
		"3?BB\u0002\u000012BB\u0398\u0000q\u0001\u0000\u0000\u0000\u0002\u0080"+
		"\u0001\u0000\u0000\u0000\u0004\u0082\u0001\u0000\u0000\u0000\u0006\u0088"+
		"\u0001\u0000\u0000\u0000\b\u009a\u0001\u0000\u0000\u0000\n\u009c\u0001"+
		"\u0000\u0000\u0000\f\u00cf\u0001\u0000\u0000\u0000\u000e\u00d1\u0001\u0000"+
		"\u0000\u0000\u0010\u016e\u0001\u0000\u0000\u0000\u0012\u0170\u0001\u0000"+
		"\u0000\u0000\u0014\u0172\u0001\u0000\u0000\u0000\u0016\u0179\u0001\u0000"+
		"\u0000\u0000\u0018\u0180\u0001\u0000\u0000\u0000\u001a\u0182\u0001\u0000"+
		"\u0000\u0000\u001c\u0184\u0001\u0000\u0000\u0000\u001e\u0188\u0001\u0000"+
		"\u0000\u0000 \u018a\u0001\u0000\u0000\u0000\"\u01b0\u0001\u0000\u0000"+
		"\u0000$\u01be\u0001\u0000\u0000\u0000&\u01c9\u0001\u0000\u0000\u0000("+
		"\u01cb\u0001\u0000\u0000\u0000*\u01d1\u0001\u0000\u0000\u0000,\u01d7\u0001"+
		"\u0000\u0000\u0000.\u01dd\u0001\u0000\u0000\u00000\u01e3\u0001\u0000\u0000"+
		"\u00002\u01e9\u0001\u0000\u0000\u00004\u01ef\u0001\u0000\u0000\u00006"+
		"\u01f5\u0001\u0000\u0000\u00008\u01fb\u0001\u0000\u0000\u0000:\u0206\u0001"+
		"\u0000\u0000\u0000<\u020b\u0001\u0000\u0000\u0000>\u0216\u0001\u0000\u0000"+
		"\u0000@\u021f\u0001\u0000\u0000\u0000B\u0223\u0001\u0000\u0000\u0000D"+
		"\u0225\u0001\u0000\u0000\u0000F\u022b\u0001\u0000\u0000\u0000H\u0234\u0001"+
		"\u0000\u0000\u0000J\u0236\u0001\u0000\u0000\u0000L\u0276\u0001\u0000\u0000"+
		"\u0000N\u0278\u0001\u0000\u0000\u0000P\u027e\u0001\u0000\u0000\u0000R"+
		"\u0286\u0001\u0000\u0000\u0000T\u0294\u0001\u0000\u0000\u0000V\u029b\u0001"+
		"\u0000\u0000\u0000X\u02ca\u0001\u0000\u0000\u0000Z\u02d3\u0001\u0000\u0000"+
		"\u0000\\\u02d6\u0001\u0000\u0000\u0000^\u02e8\u0001\u0000\u0000\u0000"+
		"`\u02f3\u0001\u0000\u0000\u0000b\u0322\u0001\u0000\u0000\u0000d\u0324"+
		"\u0001\u0000\u0000\u0000f\u033f\u0001\u0000\u0000\u0000h\u034a\u0001\u0000"+
		"\u0000\u0000j\u034c\u0001\u0000\u0000\u0000l\u034e\u0001\u0000\u0000\u0000"+
		"np\u0003\u0002\u0001\u0000on\u0001\u0000\u0000\u0000ps\u0001\u0000\u0000"+
		"\u0000qo\u0001\u0000\u0000\u0000qr\u0001\u0000\u0000\u0000rt\u0001\u0000"+
		"\u0000\u0000sq\u0001\u0000\u0000\u0000tu\u0005\u0000\u0000\u0001u\u0001"+
		"\u0001\u0000\u0000\u0000v\u0081\u0003\u0004\u0002\u0000w\u0081\u0003\u0006"+
		"\u0003\u0000x\u0081\u0003&\u0013\u0000y\u0081\u0003\u000e\u0007\u0000"+
		"z\u0081\u0003 \u0010\u0000{\u0081\u0003B!\u0000|\u0081\u0003J%\u0000}"+
		"\u0081\u0003V+\u0000~\u0081\u0003l6\u0000\u007f\u0081\u0005E\u0000\u0000"+
		"\u0080v\u0001\u0000\u0000\u0000\u0080w\u0001\u0000\u0000\u0000\u0080x"+
		"\u0001\u0000\u0000\u0000\u0080y\u0001\u0000\u0000\u0000\u0080z\u0001\u0000"+
		"\u0000\u0000\u0080{\u0001\u0000\u0000\u0000\u0080|\u0001\u0000\u0000\u0000"+
		"\u0080}\u0001\u0000\u0000\u0000\u0080~\u0001\u0000\u0000\u0000\u0080\u007f"+
		"\u0001\u0000\u0000\u0000\u0081\u0003\u0001\u0000\u0000\u0000\u0082\u0083"+
		"\u0005\r\u0000\u0000\u0083\u0084\u0003@ \u0000\u0084\u0085\u0005A\u0000"+
		"\u0000\u0085\u0086\u0005D\u0000\u0000\u0086\u0087\u0005@\u0000\u0000\u0087"+
		"\u0005\u0001\u0000\u0000\u0000\u0088\u0089\u0003\b\u0004\u0000\u0089\u008d"+
		"\u0005\u0001\u0000\u0000\u008a\u008c\u0005E\u0000\u0000\u008b\u008a\u0001"+
		"\u0000\u0000\u0000\u008c\u008f\u0001\u0000\u0000\u0000\u008d\u008b\u0001"+
		"\u0000\u0000\u0000\u008d\u008e\u0001\u0000\u0000\u0000\u008e\u0090\u0001"+
		"\u0000\u0000\u0000\u008f\u008d\u0001\u0000\u0000\u0000\u0090\u0094\u0003"+
		"\n\u0005\u0000\u0091\u0093\u0005E\u0000\u0000\u0092\u0091\u0001\u0000"+
		"\u0000\u0000\u0093\u0096\u0001\u0000\u0000\u0000\u0094\u0092\u0001\u0000"+
		"\u0000\u0000\u0094\u0095\u0001\u0000\u0000\u0000\u0095\u0097\u0001\u0000"+
		"\u0000\u0000\u0096\u0094\u0001\u0000\u0000\u0000\u0097\u0098\u0005\u0002"+
		"\u0000\u0000\u0098\u0099\u0005@\u0000\u0000\u0099\u0007\u0001\u0000\u0000"+
		"\u0000\u009a\u009b\u0007\u0000\u0000\u0000\u009b\t\u0001\u0000\u0000\u0000"+
		"\u009c\u00a7\u0003\f\u0006\u0000\u009d\u00a1\u0005\u0003\u0000\u0000\u009e"+
		"\u00a0\u0005E\u0000\u0000\u009f\u009e\u0001\u0000\u0000\u0000\u00a0\u00a3"+
		"\u0001\u0000\u0000\u0000\u00a1\u009f\u0001\u0000\u0000\u0000\u00a1\u00a2"+
		"\u0001\u0000\u0000\u0000\u00a2\u00a4\u0001\u0000\u0000\u0000\u00a3\u00a1"+
		"\u0001\u0000\u0000\u0000\u00a4\u00a6\u0003\f\u0006\u0000\u00a5\u009d\u0001"+
		"\u0000\u0000\u0000\u00a6\u00a9\u0001\u0000\u0000\u0000\u00a7\u00a5\u0001"+
		"\u0000\u0000\u0000\u00a7\u00a8\u0001\u0000\u0000\u0000\u00a8\u000b\u0001"+
		"\u0000\u0000\u0000\u00a9\u00a7\u0001\u0000\u0000\u0000\u00aa\u00ab\u0005"+
		"\u0017\u0000\u0000\u00ab\u00ac\u0005A\u0000\u0000\u00ac\u00d0\u0003<\u001e"+
		"\u0000\u00ad\u00ae\u0005\u0018\u0000\u0000\u00ae\u00af\u0005A\u0000\u0000"+
		"\u00af\u00d0\u0003<\u001e\u0000\u00b0\u00b1\u0005\u0019\u0000\u0000\u00b1"+
		"\u00b2\u0005A\u0000\u0000\u00b2\u00b6\u0005\u0004\u0000\u0000\u00b3\u00b5"+
		"\u0005E\u0000\u0000\u00b4\u00b3\u0001\u0000\u0000\u0000\u00b5\u00b8\u0001"+
		"\u0000\u0000\u0000\u00b6\u00b4\u0001\u0000\u0000\u0000\u00b6\u00b7\u0001"+
		"\u0000\u0000\u0000\u00b7\u00b9\u0001\u0000\u0000\u0000\u00b8\u00b6\u0001"+
		"\u0000\u0000\u0000\u00b9\u00c4\u0003@ \u0000\u00ba\u00be\u0005\u0003\u0000"+
		"\u0000\u00bb\u00bd\u0005E\u0000\u0000\u00bc\u00bb\u0001\u0000\u0000\u0000"+
		"\u00bd\u00c0\u0001\u0000\u0000\u0000\u00be\u00bc\u0001\u0000\u0000\u0000"+
		"\u00be\u00bf\u0001\u0000\u0000\u0000\u00bf\u00c1\u0001\u0000\u0000\u0000"+
		"\u00c0\u00be\u0001\u0000\u0000\u0000\u00c1\u00c3\u0003@ \u0000\u00c2\u00ba"+
		"\u0001\u0000\u0000\u0000\u00c3\u00c6\u0001\u0000\u0000\u0000\u00c4\u00c2"+
		"\u0001\u0000\u0000\u0000\u00c4\u00c5\u0001\u0000\u0000\u0000\u00c5\u00ca"+
		"\u0001\u0000\u0000\u0000\u00c6\u00c4\u0001\u0000\u0000\u0000\u00c7\u00c9"+
		"\u0005E\u0000\u0000\u00c8\u00c7\u0001\u0000\u0000\u0000\u00c9\u00cc\u0001"+
		"\u0000\u0000\u0000\u00ca\u00c8\u0001\u0000\u0000\u0000\u00ca\u00cb\u0001"+
		"\u0000\u0000\u0000\u00cb\u00cd\u0001\u0000\u0000\u0000\u00cc\u00ca\u0001"+
		"\u0000\u0000\u0000\u00cd\u00ce\u0005\u0005\u0000\u0000\u00ce\u00d0\u0001"+
		"\u0000\u0000\u0000\u00cf\u00aa\u0001\u0000\u0000\u0000\u00cf\u00ad\u0001"+
		"\u0000\u0000\u0000\u00cf\u00b0\u0001\u0000\u0000\u0000\u00d0\r\u0001\u0000"+
		"\u0000\u0000\u00d1\u00d2\u0005\u000e\u0000\u0000\u00d2\u00d3\u0003@ \u0000"+
		"\u00d3\u00d7\u0005\u0001\u0000\u0000\u00d4\u00d6\u0005E\u0000\u0000\u00d5"+
		"\u00d4\u0001\u0000\u0000\u0000\u00d6\u00d9\u0001\u0000\u0000\u0000\u00d7"+
		"\u00d5\u0001\u0000\u0000\u0000\u00d7\u00d8\u0001\u0000\u0000\u0000\u00d8"+
		"\u00da\u0001\u0000\u0000\u0000\u00d9\u00d7\u0001\u0000\u0000\u0000\u00da"+
		"\u00e5\u0003\u0010\b\u0000\u00db\u00df\u0005\u0003\u0000\u0000\u00dc\u00de"+
		"\u0005E\u0000\u0000\u00dd\u00dc\u0001\u0000\u0000\u0000\u00de\u00e1\u0001"+
		"\u0000\u0000\u0000\u00df\u00dd\u0001\u0000\u0000\u0000\u00df\u00e0\u0001"+
		"\u0000\u0000\u0000\u00e0\u00e2\u0001\u0000\u0000\u0000\u00e1\u00df\u0001"+
		"\u0000\u0000\u0000\u00e2\u00e4\u0003\u0010\b\u0000\u00e3\u00db\u0001\u0000"+
		"\u0000\u0000\u00e4\u00e7\u0001\u0000\u0000\u0000\u00e5\u00e3\u0001\u0000"+
		"\u0000\u0000\u00e5\u00e6\u0001\u0000\u0000\u0000\u00e6\u00eb\u0001\u0000"+
		"\u0000\u0000\u00e7\u00e5\u0001\u0000\u0000\u0000\u00e8\u00ea\u0005E\u0000"+
		"\u0000\u00e9\u00e8\u0001\u0000\u0000\u0000\u00ea\u00ed\u0001\u0000\u0000"+
		"\u0000\u00eb\u00e9\u0001\u0000\u0000\u0000\u00eb\u00ec\u0001\u0000\u0000"+
		"\u0000\u00ec\u00ee\u0001\u0000\u0000\u0000\u00ed\u00eb\u0001\u0000\u0000"+
		"\u0000\u00ee\u00ef\u0005\u0002\u0000\u0000\u00ef\u00f0\u0005@\u0000\u0000"+
		"\u00f0\u000f\u0001\u0000\u0000\u0000\u00f1\u00f2\u0005\u000f\u0000\u0000"+
		"\u00f2\u00f3\u0005A\u0000\u0000\u00f3\u00f7\u0005\u0004\u0000\u0000\u00f4"+
		"\u00f6\u0005E\u0000\u0000\u00f5\u00f4\u0001\u0000\u0000\u0000\u00f6\u00f9"+
		"\u0001\u0000\u0000\u0000\u00f7\u00f5\u0001\u0000\u0000\u0000\u00f7\u00f8"+
		"\u0001\u0000\u0000\u0000\u00f8\u00fa\u0001\u0000\u0000\u0000\u00f9\u00f7"+
		"\u0001\u0000\u0000\u0000\u00fa\u0105\u0003\u0012\t\u0000\u00fb\u00ff\u0005"+
		"\u0003\u0000\u0000\u00fc\u00fe\u0005E\u0000\u0000\u00fd\u00fc\u0001\u0000"+
		"\u0000\u0000\u00fe\u0101\u0001\u0000\u0000\u0000\u00ff\u00fd\u0001\u0000"+
		"\u0000\u0000\u00ff\u0100\u0001\u0000\u0000\u0000\u0100\u0102\u0001\u0000"+
		"\u0000\u0000\u0101\u00ff\u0001\u0000\u0000\u0000\u0102\u0104\u0003\u0012"+
		"\t\u0000\u0103\u00fb\u0001\u0000\u0000\u0000\u0104\u0107\u0001\u0000\u0000"+
		"\u0000\u0105\u0103\u0001\u0000\u0000\u0000\u0105\u0106\u0001\u0000\u0000"+
		"\u0000\u0106\u010b\u0001\u0000\u0000\u0000\u0107\u0105\u0001\u0000\u0000"+
		"\u0000\u0108\u010a\u0005E\u0000\u0000\u0109\u0108\u0001\u0000\u0000\u0000"+
		"\u010a\u010d\u0001\u0000\u0000\u0000\u010b\u0109\u0001\u0000\u0000\u0000"+
		"\u010b\u010c\u0001\u0000\u0000\u0000\u010c\u010e\u0001\u0000\u0000\u0000"+
		"\u010d\u010b\u0001\u0000\u0000\u0000\u010e\u010f\u0005\u0005\u0000\u0000"+
		"\u010f\u016f\u0001\u0000\u0000\u0000\u0110\u0111\u0005\u0011\u0000\u0000"+
		"\u0111\u0112\u0005A\u0000\u0000\u0112\u0116\u0005\u0004\u0000\u0000\u0113"+
		"\u0115\u0005E\u0000\u0000\u0114\u0113\u0001\u0000\u0000\u0000\u0115\u0118"+
		"\u0001\u0000\u0000\u0000\u0116\u0114\u0001\u0000\u0000\u0000\u0116\u0117"+
		"\u0001\u0000\u0000\u0000\u0117\u0119\u0001\u0000\u0000\u0000\u0118\u0116"+
		"\u0001\u0000\u0000\u0000\u0119\u0124\u0003\u0014\n\u0000\u011a\u011e\u0005"+
		"\u0003\u0000\u0000\u011b\u011d\u0005E\u0000\u0000\u011c\u011b\u0001\u0000"+
		"\u0000\u0000\u011d\u0120\u0001\u0000\u0000\u0000\u011e\u011c\u0001\u0000"+
		"\u0000\u0000\u011e\u011f\u0001\u0000\u0000\u0000\u011f\u0121\u0001\u0000"+
		"\u0000\u0000\u0120\u011e\u0001\u0000\u0000\u0000\u0121\u0123\u0003\u0014"+
		"\n\u0000\u0122\u011a\u0001\u0000\u0000\u0000\u0123\u0126\u0001\u0000\u0000"+
		"\u0000\u0124\u0122\u0001\u0000\u0000\u0000\u0124\u0125\u0001\u0000\u0000"+
		"\u0000\u0125\u012a\u0001\u0000\u0000\u0000\u0126\u0124\u0001\u0000\u0000"+
		"\u0000\u0127\u0129\u0005E\u0000\u0000\u0128\u0127\u0001\u0000\u0000\u0000"+
		"\u0129\u012c\u0001\u0000\u0000\u0000\u012a\u0128\u0001\u0000\u0000\u0000"+
		"\u012a\u012b\u0001\u0000\u0000\u0000\u012b\u012d\u0001\u0000\u0000\u0000"+
		"\u012c\u012a\u0001\u0000\u0000\u0000\u012d\u012e\u0005\u0005\u0000\u0000"+
		"\u012e\u016f\u0001\u0000\u0000\u0000\u012f\u0130\u0005\u0012\u0000\u0000"+
		"\u0130\u0131\u0005A\u0000\u0000\u0131\u0135\u0005\u0004\u0000\u0000\u0132"+
		"\u0134\u0005E\u0000\u0000\u0133\u0132\u0001\u0000\u0000\u0000\u0134\u0137"+
		"\u0001\u0000\u0000\u0000\u0135\u0133\u0001\u0000\u0000\u0000\u0135\u0136"+
		"\u0001\u0000\u0000\u0000\u0136\u0138\u0001\u0000\u0000\u0000\u0137\u0135"+
		"\u0001\u0000\u0000\u0000\u0138\u0143\u0003\u0016\u000b\u0000\u0139\u013d"+
		"\u0005\u0003\u0000\u0000\u013a\u013c\u0005E\u0000\u0000\u013b\u013a\u0001"+
		"\u0000\u0000\u0000\u013c\u013f\u0001\u0000\u0000\u0000\u013d\u013b\u0001"+
		"\u0000\u0000\u0000\u013d\u013e\u0001\u0000\u0000\u0000\u013e\u0140\u0001"+
		"\u0000\u0000\u0000\u013f\u013d\u0001\u0000\u0000\u0000\u0140\u0142\u0003"+
		"\u0016\u000b\u0000\u0141\u0139\u0001\u0000\u0000\u0000\u0142\u0145\u0001"+
		"\u0000\u0000\u0000\u0143\u0141\u0001\u0000\u0000\u0000\u0143\u0144\u0001"+
		"\u0000\u0000\u0000\u0144\u0149\u0001\u0000\u0000\u0000\u0145\u0143\u0001"+
		"\u0000\u0000\u0000\u0146\u0148\u0005E\u0000\u0000\u0147\u0146\u0001\u0000"+
		"\u0000\u0000\u0148\u014b\u0001\u0000\u0000\u0000\u0149\u0147\u0001\u0000"+
		"\u0000\u0000\u0149\u014a\u0001\u0000\u0000\u0000\u014a\u014c\u0001\u0000"+
		"\u0000\u0000\u014b\u0149\u0001\u0000\u0000\u0000\u014c\u014d\u0005\u0005"+
		"\u0000\u0000\u014d\u016f\u0001\u0000\u0000\u0000\u014e\u014f\u0005\u0014"+
		"\u0000\u0000\u014f\u0150\u0005A\u0000\u0000\u0150\u0154\u0005\u0004\u0000"+
		"\u0000\u0151\u0153\u0005E\u0000\u0000\u0152\u0151\u0001\u0000\u0000\u0000"+
		"\u0153\u0156\u0001\u0000\u0000\u0000\u0154\u0152\u0001\u0000\u0000\u0000"+
		"\u0154\u0155\u0001\u0000\u0000\u0000\u0155\u0157\u0001\u0000\u0000\u0000"+
		"\u0156\u0154\u0001\u0000\u0000\u0000\u0157\u0162\u0003\u001a\r\u0000\u0158"+
		"\u015c\u0005\u0003\u0000\u0000\u0159\u015b\u0005E\u0000\u0000\u015a\u0159"+
		"\u0001\u0000\u0000\u0000\u015b\u015e\u0001\u0000\u0000\u0000\u015c\u015a"+
		"\u0001\u0000\u0000\u0000\u015c\u015d\u0001\u0000\u0000\u0000\u015d\u015f"+
		"\u0001\u0000\u0000\u0000\u015e\u015c\u0001\u0000\u0000\u0000\u015f\u0161"+
		"\u0003\u001a\r\u0000\u0160\u0158\u0001\u0000\u0000\u0000\u0161\u0164\u0001"+
		"\u0000\u0000\u0000\u0162\u0160\u0001\u0000\u0000\u0000\u0162\u0163\u0001"+
		"\u0000\u0000\u0000\u0163\u0168\u0001\u0000\u0000\u0000\u0164\u0162\u0001"+
		"\u0000\u0000\u0000\u0165\u0167\u0005E\u0000\u0000\u0166\u0165\u0001\u0000"+
		"\u0000\u0000\u0167\u016a\u0001\u0000\u0000\u0000\u0168\u0166\u0001\u0000"+
		"\u0000\u0000\u0168\u0169\u0001\u0000\u0000\u0000\u0169\u016b\u0001\u0000"+
		"\u0000\u0000\u016a\u0168\u0001\u0000\u0000\u0000\u016b\u016c\u0005\u0005"+
		"\u0000\u0000\u016c\u016f\u0001\u0000\u0000\u0000\u016d\u016f\u0003\u001c"+
		"\u000e\u0000\u016e\u00f1\u0001\u0000\u0000\u0000\u016e\u0110\u0001\u0000"+
		"\u0000\u0000\u016e\u012f\u0001\u0000\u0000\u0000\u016e\u014e\u0001\u0000"+
		"\u0000\u0000\u016e\u016d\u0001\u0000\u0000\u0000\u016f\u0011\u0001\u0000"+
		"\u0000\u0000\u0170\u0171\u0003@ \u0000\u0171\u0013\u0001\u0000\u0000\u0000"+
		"\u0172\u0173\u0003@ \u0000\u0173\u0175\u0005\u0006\u0000\u0000\u0174\u0176"+
		"\u0005\u0013\u0000\u0000\u0175\u0174\u0001\u0000\u0000\u0000\u0175\u0176"+
		"\u0001\u0000\u0000\u0000\u0176\u0177\u0001\u0000\u0000\u0000\u0177\u0178"+
		"\u0003\u0018\f\u0000\u0178\u0015\u0001\u0000\u0000\u0000\u0179\u017a\u0003"+
		"@ \u0000\u017a\u017c\u0005\u0006\u0000\u0000\u017b\u017d\u0005\u0013\u0000"+
		"\u0000\u017c\u017b\u0001\u0000\u0000\u0000\u017c\u017d\u0001\u0000\u0000"+
		"\u0000\u017d\u017e\u0001\u0000\u0000\u0000\u017e\u017f\u0005&\u0000\u0000"+
		"\u017f\u0017\u0001\u0000\u0000\u0000\u0180\u0181\u0007\u0001\u0000\u0000"+
		"\u0181\u0019\u0001\u0000\u0000\u0000\u0182\u0183\u0003@ \u0000\u0183\u001b"+
		"\u0001\u0000\u0000\u0000\u0184\u0185\u0005\u0010\u0000\u0000\u0185\u0186"+
		"\u0005A\u0000\u0000\u0186\u0187\u0003\u001e\u000f\u0000\u0187\u001d\u0001"+
		"\u0000\u0000\u0000\u0188\u0189\u0007\u0002\u0000\u0000\u0189\u001f\u0001"+
		"\u0000\u0000\u0000\u018a\u018b\u0005\u001a\u0000\u0000\u018b\u018c\u0005"+
		"B\u0000\u0000\u018c\u0190\u0005\u0001\u0000\u0000\u018d\u018f\u0005E\u0000"+
		"\u0000\u018e\u018d\u0001\u0000\u0000\u0000\u018f\u0192\u0001\u0000\u0000"+
		"\u0000\u0190\u018e\u0001\u0000\u0000\u0000\u0190\u0191\u0001\u0000\u0000"+
		"\u0000\u0191\u0193\u0001\u0000\u0000\u0000\u0192\u0190\u0001\u0000\u0000"+
		"\u0000\u0193\u0194\u0005\u001b\u0000\u0000\u0194\u0195\u0005A\u0000\u0000"+
		"\u0195\u0199\u0005\u0004\u0000\u0000\u0196\u0198\u0005E\u0000\u0000\u0197"+
		"\u0196\u0001\u0000\u0000\u0000\u0198\u019b\u0001\u0000\u0000\u0000\u0199"+
		"\u0197\u0001\u0000\u0000\u0000\u0199\u019a\u0001\u0000\u0000\u0000\u019a"+
		"\u019c\u0001\u0000\u0000\u0000\u019b\u0199\u0001\u0000\u0000\u0000\u019c"+
		"\u01a0\u0003\"\u0011\u0000\u019d\u019f\u0005E\u0000\u0000\u019e\u019d"+
		"\u0001\u0000\u0000\u0000\u019f\u01a2\u0001\u0000\u0000\u0000\u01a0\u019e"+
		"\u0001\u0000\u0000\u0000\u01a0\u01a1\u0001\u0000\u0000\u0000\u01a1\u01a3"+
		"\u0001\u0000\u0000\u0000\u01a2\u01a0\u0001\u0000\u0000\u0000\u01a3\u01a5"+
		"\u0005\u0005\u0000\u0000\u01a4\u01a6\u0005\u0003\u0000\u0000\u01a5\u01a4"+
		"\u0001\u0000\u0000\u0000\u01a5\u01a6\u0001\u0000\u0000\u0000\u01a6\u01aa"+
		"\u0001\u0000\u0000\u0000\u01a7\u01a9\u0005E\u0000\u0000\u01a8\u01a7\u0001"+
		"\u0000\u0000\u0000\u01a9\u01ac\u0001\u0000\u0000\u0000\u01aa\u01a8\u0001"+
		"\u0000\u0000\u0000\u01aa\u01ab\u0001\u0000\u0000\u0000\u01ab\u01ad\u0001"+
		"\u0000\u0000\u0000\u01ac\u01aa\u0001\u0000\u0000\u0000\u01ad\u01ae\u0005"+
		"\u0002\u0000\u0000\u01ae\u01af\u0005@\u0000\u0000\u01af!\u0001\u0000\u0000"+
		"\u0000\u01b0\u01bb\u0003$\u0012\u0000\u01b1\u01b5\u0005\u0003\u0000\u0000"+
		"\u01b2\u01b4\u0005E\u0000\u0000\u01b3\u01b2\u0001\u0000\u0000\u0000\u01b4"+
		"\u01b7\u0001\u0000\u0000\u0000\u01b5\u01b3\u0001\u0000\u0000\u0000\u01b5"+
		"\u01b6\u0001\u0000\u0000\u0000\u01b6\u01b8\u0001\u0000\u0000\u0000\u01b7"+
		"\u01b5\u0001\u0000\u0000\u0000\u01b8\u01ba\u0003$\u0012\u0000\u01b9\u01b1"+
		"\u0001\u0000\u0000\u0000\u01ba\u01bd\u0001\u0000\u0000\u0000\u01bb\u01b9"+
		"\u0001\u0000\u0000\u0000\u01bb\u01bc\u0001\u0000\u0000\u0000\u01bc#\u0001"+
		"\u0000\u0000\u0000\u01bd\u01bb\u0001\u0000\u0000\u0000\u01be\u01bf\u0005"+
		"D\u0000\u0000\u01bf%\u0001\u0000\u0000\u0000\u01c0\u01ca\u0003(\u0014"+
		"\u0000\u01c1\u01ca\u0003*\u0015\u0000\u01c2\u01ca\u0003,\u0016\u0000\u01c3"+
		"\u01ca\u0003.\u0017\u0000\u01c4\u01ca\u00030\u0018\u0000\u01c5\u01ca\u0003"+
		"2\u0019\u0000\u01c6\u01ca\u00034\u001a\u0000\u01c7\u01ca\u00036\u001b"+
		"\u0000\u01c8\u01ca\u00038\u001c\u0000\u01c9\u01c0\u0001\u0000\u0000\u0000"+
		"\u01c9\u01c1\u0001\u0000\u0000\u0000\u01c9\u01c2\u0001\u0000\u0000\u0000"+
		"\u01c9\u01c3\u0001\u0000\u0000\u0000\u01c9\u01c4\u0001\u0000\u0000\u0000"+
		"\u01c9\u01c5\u0001\u0000\u0000\u0000\u01c9\u01c6\u0001\u0000\u0000\u0000"+
		"\u01c9\u01c7\u0001\u0000\u0000\u0000\u01c9\u01c8\u0001\u0000\u0000\u0000"+
		"\u01ca\'\u0001\u0000\u0000\u0000\u01cb\u01cc\u0005\u001d\u0000\u0000\u01cc"+
		"\u01cd\u0003@ \u0000\u01cd\u01ce\u0005A\u0000\u0000\u01ce\u01cf\u0003"+
		":\u001d\u0000\u01cf\u01d0\u0005@\u0000\u0000\u01d0)\u0001\u0000\u0000"+
		"\u0000\u01d1\u01d2\u0005\u001e\u0000\u0000\u01d2\u01d3\u0003@ \u0000\u01d3"+
		"\u01d4\u0005A\u0000\u0000\u01d4\u01d5\u0003<\u001e\u0000\u01d5\u01d6\u0005"+
		"@\u0000\u0000\u01d6+\u0001\u0000\u0000\u0000\u01d7\u01d8\u0005\u001f\u0000"+
		"\u0000\u01d8\u01d9\u0003@ \u0000\u01d9\u01da\u0005A\u0000\u0000\u01da"+
		"\u01db\u0003:\u001d\u0000\u01db\u01dc\u0005@\u0000\u0000\u01dc-\u0001"+
		"\u0000\u0000\u0000\u01dd\u01de\u0005 \u0000\u0000\u01de\u01df\u0003@ "+
		"\u0000\u01df\u01e0\u0005A\u0000\u0000\u01e0\u01e1\u0003<\u001e\u0000\u01e1"+
		"\u01e2\u0005@\u0000\u0000\u01e2/\u0001\u0000\u0000\u0000\u01e3\u01e4\u0005"+
		"!\u0000\u0000\u01e4\u01e5\u0003@ \u0000\u01e5\u01e6\u0005A\u0000\u0000"+
		"\u01e6\u01e7\u0003>\u001f\u0000\u01e7\u01e8\u0005@\u0000\u0000\u01e81"+
		"\u0001\u0000\u0000\u0000\u01e9\u01ea\u0005\"\u0000\u0000\u01ea\u01eb\u0003"+
		"@ \u0000\u01eb\u01ec\u0005A\u0000\u0000\u01ec\u01ed\u0005D\u0000\u0000"+
		"\u01ed\u01ee\u0005@\u0000\u0000\u01ee3\u0001\u0000\u0000\u0000\u01ef\u01f0"+
		"\u0005#\u0000\u0000\u01f0\u01f1\u0003@ \u0000\u01f1\u01f2\u0005A\u0000"+
		"\u0000\u01f2\u01f3\u0005D\u0000\u0000\u01f3\u01f4\u0005@\u0000\u0000\u01f4"+
		"5\u0001\u0000\u0000\u0000\u01f5\u01f6\u0005$\u0000\u0000\u01f6\u01f7\u0003"+
		"@ \u0000\u01f7\u01f8\u0005A\u0000\u0000\u01f8\u01f9\u0005D\u0000\u0000"+
		"\u01f9\u01fa\u0005@\u0000\u0000\u01fa7\u0001\u0000\u0000\u0000\u01fb\u01fc"+
		"\u0005%\u0000\u0000\u01fc\u01fd\u0003@ \u0000\u01fd\u01fe\u0005A\u0000"+
		"\u0000\u01fe\u01ff\u0005D\u0000\u0000\u01ff\u0200\u0005@\u0000\u0000\u0200"+
		"9\u0001\u0000\u0000\u0000\u0201\u0202\u0005:\u0000\u0000\u0202\u0207\u0003"+
		":\u001d\u0000\u0203\u0204\u0005;\u0000\u0000\u0204\u0207\u0003:\u001d"+
		"\u0000\u0205\u0207\u0005C\u0000\u0000\u0206\u0201\u0001\u0000\u0000\u0000"+
		"\u0206\u0203\u0001\u0000\u0000\u0000\u0206\u0205\u0001\u0000\u0000\u0000"+
		"\u0207;\u0001\u0000\u0000\u0000\u0208\u0209\u0005:\u0000\u0000\u0209\u020c"+
		"\u0003<\u001e\u0000\u020a\u020c\u0005C\u0000\u0000\u020b\u0208\u0001\u0000"+
		"\u0000\u0000\u020b\u020a\u0001\u0000\u0000\u0000\u020c=\u0001\u0000\u0000"+
		"\u0000\u020d\u020e\u0005:\u0000\u0000\u020e\u0217\u0003>\u001f\u0000\u020f"+
		"\u0210\u0005;\u0000\u0000\u0210\u0217\u0003>\u001f\u0000\u0211\u0214\u0005"+
		"C\u0000\u0000\u0212\u0213\u0005\u0007\u0000\u0000\u0213\u0215\u0005C\u0000"+
		"\u0000\u0214\u0212\u0001\u0000\u0000\u0000\u0214\u0215\u0001\u0000\u0000"+
		"\u0000\u0215\u0217\u0001\u0000\u0000\u0000\u0216\u020d\u0001\u0000\u0000"+
		"\u0000\u0216\u020f\u0001\u0000\u0000\u0000\u0216\u0211\u0001\u0000\u0000"+
		"\u0000\u0217?\u0001\u0000\u0000\u0000\u0218\u0219\u0005B\u0000\u0000\u0219"+
		"\u021a\u0005\b\u0000\u0000\u021a\u0220\u0005B\u0000\u0000\u021b\u021c"+
		"\u0005B\u0000\u0000\u021c\u021d\u0005\b\u0000\u0000\u021d\u0220\u0005"+
		"D\u0000\u0000\u021e\u0220\u0005B\u0000\u0000\u021f\u0218\u0001\u0000\u0000"+
		"\u0000\u021f\u021b\u0001\u0000\u0000\u0000\u021f\u021e\u0001\u0000\u0000"+
		"\u0000\u0220A\u0001\u0000\u0000\u0000\u0221\u0224\u0003D\"\u0000\u0222"+
		"\u0224\u0003F#\u0000\u0223\u0221\u0001\u0000\u0000\u0000\u0223\u0222\u0001"+
		"\u0000\u0000\u0000\u0224C\u0001\u0000\u0000\u0000\u0225\u0226\u0005&\u0000"+
		"\u0000\u0226\u0227\u0003@ \u0000\u0227\u0228\u0005A\u0000\u0000\u0228"+
		"\u0229\u0003H$\u0000\u0229\u022a\u0005@\u0000\u0000\u022aE\u0001\u0000"+
		"\u0000\u0000\u022b\u022c\u0005\'\u0000\u0000\u022c\u022d\u0003@ \u0000"+
		"\u022d\u022e\u0005A\u0000\u0000\u022e\u022f\u0005D\u0000\u0000\u022f\u0230"+
		"\u0005@\u0000\u0000\u0230G\u0001\u0000\u0000\u0000\u0231\u0235\u0003d"+
		"2\u0000\u0232\u0235\u0005(\u0000\u0000\u0233\u0235\u0005D\u0000\u0000"+
		"\u0234\u0231\u0001\u0000\u0000\u0000\u0234\u0232\u0001\u0000\u0000\u0000"+
		"\u0234\u0233\u0001\u0000\u0000\u0000\u0235I\u0001\u0000\u0000\u0000\u0236"+
		"\u0237\u0005)\u0000\u0000\u0237\u0238\u0003@ \u0000\u0238\u023c\u0005"+
		"\u0001\u0000\u0000\u0239\u023b\u0005E\u0000\u0000\u023a\u0239\u0001\u0000"+
		"\u0000\u0000\u023b\u023e\u0001\u0000\u0000\u0000\u023c\u023a\u0001\u0000"+
		"\u0000\u0000\u023c\u023d\u0001\u0000\u0000\u0000\u023d\u023f\u0001\u0000"+
		"\u0000\u0000\u023e\u023c\u0001\u0000\u0000\u0000\u023f\u0243\u0003L&\u0000"+
		"\u0240\u0242\u0005E\u0000\u0000\u0241\u0240\u0001\u0000\u0000\u0000\u0242"+
		"\u0245\u0001\u0000\u0000\u0000\u0243\u0241\u0001\u0000\u0000\u0000\u0243"+
		"\u0244\u0001\u0000\u0000\u0000\u0244\u0246\u0001\u0000\u0000\u0000\u0245"+
		"\u0243\u0001\u0000\u0000\u0000\u0246\u0247\u0005,\u0000\u0000\u0247\u0248"+
		"\u0005A\u0000\u0000\u0248\u0249\u0003N\'\u0000\u0249\u024d\u0005\u0003"+
		"\u0000\u0000\u024a\u024c\u0005E\u0000\u0000\u024b\u024a\u0001\u0000\u0000"+
		"\u0000\u024c\u024f\u0001\u0000\u0000\u0000\u024d\u024b\u0001\u0000\u0000"+
		"\u0000\u024d\u024e\u0001\u0000\u0000\u0000\u024e\u0250\u0001\u0000\u0000"+
		"\u0000\u024f\u024d\u0001\u0000\u0000\u0000\u0250\u0251\u0005-\u0000\u0000"+
		"\u0251\u0252\u0005A\u0000\u0000\u0252\u0256\u0005\u0004\u0000\u0000\u0253"+
		"\u0255\u0005E\u0000\u0000\u0254\u0253\u0001\u0000\u0000\u0000\u0255\u0258"+
		"\u0001\u0000\u0000\u0000\u0256\u0254\u0001\u0000\u0000\u0000\u0256\u0257"+
		"\u0001\u0000\u0000\u0000\u0257\u0259\u0001\u0000\u0000\u0000\u0258\u0256"+
		"\u0001\u0000\u0000\u0000\u0259\u025d\u0003R)\u0000\u025a\u025c\u0005E"+
		"\u0000\u0000\u025b\u025a\u0001\u0000\u0000\u0000\u025c\u025f\u0001\u0000"+
		"\u0000\u0000\u025d\u025b\u0001\u0000\u0000\u0000\u025d\u025e\u0001\u0000"+
		"\u0000\u0000\u025e\u0260\u0001\u0000\u0000\u0000\u025f\u025d\u0001\u0000"+
		"\u0000\u0000\u0260\u0262\u0005\u0005\u0000\u0000\u0261\u0263\u0005\u0003"+
		"\u0000\u0000\u0262\u0261\u0001\u0000\u0000\u0000\u0262\u0263\u0001\u0000"+
		"\u0000\u0000\u0263\u0267\u0001\u0000\u0000\u0000\u0264\u0266\u0005E\u0000"+
		"\u0000\u0265\u0264\u0001\u0000\u0000\u0000\u0266\u0269\u0001\u0000\u0000"+
		"\u0000\u0267\u0265\u0001\u0000\u0000\u0000\u0267\u0268\u0001\u0000\u0000"+
		"\u0000\u0268\u026a\u0001\u0000\u0000\u0000\u0269\u0267\u0001\u0000\u0000"+
		"\u0000\u026a\u026b\u0005\u0002\u0000\u0000\u026b\u026c\u0005@\u0000\u0000"+
		"\u026cK\u0001\u0000\u0000\u0000\u026d\u026e\u0005*\u0000\u0000\u026e\u026f"+
		"\u0005A\u0000\u0000\u026f\u0270\u0003@ \u0000\u0270\u0271\u0005\u0003"+
		"\u0000\u0000\u0271\u0277\u0001\u0000\u0000\u0000\u0272\u0273\u0005+\u0000"+
		"\u0000\u0273\u0274\u0005A\u0000\u0000\u0274\u0275\u0005D\u0000\u0000\u0275"+
		"\u0277\u0005\u0003\u0000\u0000\u0276\u026d\u0001\u0000\u0000\u0000\u0276"+
		"\u0272\u0001\u0000\u0000\u0000\u0277M\u0001\u0000\u0000\u0000\u0278\u027a"+
		"\u0005\u0004\u0000\u0000\u0279\u027b\u0003P(\u0000\u027a\u0279\u0001\u0000"+
		"\u0000\u0000\u027a\u027b\u0001\u0000\u0000\u0000\u027b\u027c\u0001\u0000"+
		"\u0000\u0000\u027c\u027d\u0005\u0005\u0000\u0000\u027dO\u0001\u0000\u0000"+
		"\u0000\u027e\u0283\u0005D\u0000\u0000\u027f\u0280\u0005\u0003\u0000\u0000"+
		"\u0280\u0282\u0005D\u0000\u0000\u0281\u027f\u0001\u0000\u0000\u0000\u0282"+
		"\u0285\u0001\u0000\u0000\u0000\u0283\u0281\u0001\u0000\u0000\u0000\u0283"+
		"\u0284\u0001\u0000\u0000\u0000\u0284Q\u0001\u0000\u0000\u0000\u0285\u0283"+
		"\u0001\u0000\u0000\u0000\u0286\u0291\u0003T*\u0000\u0287\u028b\u0005\u0003"+
		"\u0000\u0000\u0288\u028a\u0005E\u0000\u0000\u0289\u0288\u0001\u0000\u0000"+
		"\u0000\u028a\u028d\u0001\u0000\u0000\u0000\u028b\u0289\u0001\u0000\u0000"+
		"\u0000\u028b\u028c\u0001\u0000\u0000\u0000\u028c\u028e\u0001\u0000\u0000"+
		"\u0000\u028d\u028b\u0001\u0000\u0000\u0000\u028e\u0290\u0003T*\u0000\u028f"+
		"\u0287\u0001\u0000\u0000\u0000\u0290\u0293\u0001\u0000\u0000\u0000\u0291"+
		"\u028f\u0001\u0000\u0000\u0000\u0291\u0292\u0001\u0000\u0000\u0000\u0292"+
		"S\u0001\u0000\u0000\u0000\u0293\u0291\u0001\u0000\u0000\u0000\u0294\u0295"+
		"\u0005D\u0000\u0000\u0295\u0297\u0005\u0006\u0000\u0000\u0296\u0298\u0005"+
		"\u0013\u0000\u0000\u0297\u0296\u0001\u0000\u0000\u0000\u0297\u0298\u0001"+
		"\u0000\u0000\u0000\u0298\u0299\u0001\u0000\u0000\u0000\u0299\u029a\u0003"+
		"\u0018\f\u0000\u029aU\u0001\u0000\u0000\u0000\u029b\u029c\u0005\u0004"+
		"\u0000\u0000\u029c\u02a0\u0005B\u0000\u0000\u029d\u029f\u0003X,\u0000"+
		"\u029e\u029d\u0001\u0000\u0000\u0000\u029f\u02a2\u0001\u0000\u0000\u0000"+
		"\u02a0\u029e\u0001\u0000\u0000\u0000\u02a0\u02a1\u0001\u0000\u0000\u0000"+
		"\u02a1\u02a3\u0001\u0000\u0000\u0000\u02a2\u02a0\u0001\u0000\u0000\u0000"+
		"\u02a3\u02a4\u0005\u0005\u0000\u0000\u02a4\u02a8\u0005\b\u0000\u0000\u02a5"+
		"\u02a7\u0005E\u0000\u0000\u02a6\u02a5\u0001\u0000\u0000\u0000\u02a7\u02aa"+
		"\u0001\u0000\u0000\u0000\u02a8\u02a6\u0001\u0000\u0000\u0000\u02a8\u02a9"+
		"\u0001\u0000\u0000\u0000\u02a9\u02b2\u0001\u0000\u0000\u0000\u02aa\u02a8"+
		"\u0001\u0000\u0000\u0000\u02ab\u02af\u0003\\.\u0000\u02ac\u02ae\u0005"+
		"E\u0000\u0000\u02ad\u02ac\u0001\u0000\u0000\u0000\u02ae\u02b1\u0001\u0000"+
		"\u0000\u0000\u02af\u02ad\u0001\u0000\u0000\u0000\u02af\u02b0\u0001\u0000"+
		"\u0000\u0000\u02b0\u02b3\u0001\u0000\u0000\u0000\u02b1\u02af\u0001\u0000"+
		"\u0000\u0000\u02b2\u02ab\u0001\u0000\u0000\u0000\u02b3\u02b4\u0001\u0000"+
		"\u0000\u0000\u02b4\u02b2\u0001\u0000\u0000\u0000\u02b4\u02b5\u0001\u0000"+
		"\u0000\u0000\u02b5\u02b6\u0001\u0000\u0000\u0000\u02b6\u02ba\u0005\t\u0000"+
		"\u0000\u02b7\u02b9\u0005E\u0000\u0000\u02b8\u02b7\u0001\u0000\u0000\u0000"+
		"\u02b9\u02bc\u0001\u0000\u0000\u0000\u02ba\u02b8\u0001\u0000\u0000\u0000"+
		"\u02ba\u02bb\u0001\u0000\u0000\u0000\u02bb\u02c4\u0001\u0000\u0000\u0000"+
		"\u02bc\u02ba\u0001\u0000\u0000\u0000\u02bd\u02c1\u0003^/\u0000\u02be\u02c0"+
		"\u0005E\u0000\u0000\u02bf\u02be\u0001\u0000\u0000\u0000\u02c0\u02c3\u0001"+
		"\u0000\u0000\u0000\u02c1\u02bf\u0001\u0000\u0000\u0000\u02c1\u02c2\u0001"+
		"\u0000\u0000\u0000\u02c2\u02c5\u0001\u0000\u0000\u0000\u02c3\u02c1\u0001"+
		"\u0000\u0000\u0000\u02c4\u02bd\u0001\u0000\u0000\u0000\u02c5\u02c6\u0001"+
		"\u0000\u0000\u0000\u02c6\u02c4\u0001\u0000\u0000\u0000\u02c6\u02c7\u0001"+
		"\u0000\u0000\u0000\u02c7\u02c8\u0001\u0000\u0000\u0000\u02c8\u02c9\u0005"+
		"@\u0000\u0000\u02c9W\u0001\u0000\u0000\u0000\u02ca\u02cb\u0005\u0003\u0000"+
		"\u0000\u02cb\u02cc\u0005B\u0000\u0000\u02cc\u02cd\u0005A\u0000\u0000\u02cd"+
		"\u02ce\u0003Z-\u0000\u02ceY\u0001\u0000\u0000\u0000\u02cf\u02d4\u0005"+
		"D\u0000\u0000\u02d0\u02d4\u0005.\u0000\u0000\u02d1\u02d4\u0005/\u0000"+
		"\u0000\u02d2\u02d4\u0003:\u001d\u0000\u02d3\u02cf\u0001\u0000\u0000\u0000"+
		"\u02d3\u02d0\u0001\u0000\u0000\u0000\u02d3\u02d1\u0001\u0000\u0000\u0000"+
		"\u02d3\u02d2\u0001\u0000\u0000\u0000\u02d4[\u0001\u0000\u0000\u0000\u02d5"+
		"\u02d7\u00051\u0000\u0000\u02d6\u02d5\u0001\u0000\u0000\u0000\u02d6\u02d7"+
		"\u0001\u0000\u0000\u0000\u02d7\u02d8\u0001\u0000\u0000\u0000\u02d8\u02d9"+
		"\u0005\n\u0000\u0000\u02d9\u02da\u0003`0\u0000\u02da\u02db\u0003`0\u0000"+
		"\u02db\u02dc\u0003b1\u0000\u02dc\u02de\u0005\u000b\u0000\u0000\u02dd\u02df"+
		"\u0005\u0007\u0000\u0000\u02de\u02dd\u0001\u0000\u0000\u0000\u02de\u02df"+
		"\u0001\u0000\u0000\u0000\u02df\u02e6\u0001\u0000\u0000\u0000\u02e0\u02e1"+
		"\u0005\u0004\u0000\u0000\u02e1\u02e2\u0003f3\u0000\u02e2\u02e4\u0005\u0005"+
		"\u0000\u0000\u02e3\u02e5\u0005\u0007\u0000\u0000\u02e4\u02e3\u0001\u0000"+
		"\u0000\u0000\u02e4\u02e5\u0001\u0000\u0000\u0000\u02e5\u02e7\u0001\u0000"+
		"\u0000\u0000\u02e6\u02e0\u0001\u0000\u0000\u0000\u02e6\u02e7\u0001\u0000"+
		"\u0000\u0000\u02e7]\u0001\u0000\u0000\u0000\u02e8\u02e9\u0005\n\u0000"+
		"\u0000\u02e9\u02ea\u0003`0\u0000\u02ea\u02eb\u0003`0\u0000\u02eb\u02ec"+
		"\u0003f3\u0000\u02ec\u02ee\u0005\u000b\u0000\u0000\u02ed\u02ef\u0005\u0007"+
		"\u0000\u0000\u02ee\u02ed\u0001\u0000\u0000\u0000\u02ee\u02ef\u0001\u0000"+
		"\u0000\u0000\u02ef_\u0001\u0000\u0000\u0000\u02f0\u02f1\u0005\f\u0000"+
		"\u0000\u02f1\u02f4\u0005B\u0000\u0000\u02f2\u02f4\u0003@ \u0000\u02f3"+
		"\u02f0\u0001\u0000\u0000\u0000\u02f3\u02f2\u0001\u0000\u0000\u0000\u02f4"+
		"a\u0001\u0000\u0000\u0000\u02f5\u0323\u0003`0\u0000\u02f6\u02f7\u0005"+
		"\u001d\u0000\u0000\u02f7\u02f8\u0005\n\u0000\u0000\u02f8\u02f9\u0003:"+
		"\u001d\u0000\u02f9\u02fa\u0005\u000b\u0000\u0000\u02fa\u0323\u0001\u0000"+
		"\u0000\u0000\u02fb\u02fc\u0005\u001e\u0000\u0000\u02fc\u02fd\u0005\n\u0000"+
		"\u0000\u02fd\u02fe\u0003<\u001e\u0000\u02fe\u02ff\u0005\u000b\u0000\u0000"+
		"\u02ff\u0323\u0001\u0000\u0000\u0000\u0300\u0301\u0005\u001f\u0000\u0000"+
		"\u0301\u0302\u0005\n\u0000\u0000\u0302\u0303\u0003:\u001d\u0000\u0303"+
		"\u0304\u0005\u000b\u0000\u0000\u0304\u0323\u0001\u0000\u0000\u0000\u0305"+
		"\u0306\u0005 \u0000\u0000\u0306\u0307\u0005\n\u0000\u0000\u0307\u0308"+
		"\u0003<\u001e\u0000\u0308\u0309\u0005\u000b\u0000\u0000\u0309\u0323\u0001"+
		"\u0000\u0000\u0000\u030a\u030b\u0005!\u0000\u0000\u030b\u030c\u0005\n"+
		"\u0000\u0000\u030c\u030d\u0003>\u001f\u0000\u030d\u030e\u0005\u000b\u0000"+
		"\u0000\u030e\u0323\u0001\u0000\u0000\u0000\u030f\u0310\u0005\"\u0000\u0000"+
		"\u0310\u0311\u0005\n\u0000\u0000\u0311\u0312\u0005D\u0000\u0000\u0312"+
		"\u0323\u0005\u000b\u0000\u0000\u0313\u0314\u0005#\u0000\u0000\u0314\u0315"+
		"\u0005\n\u0000\u0000\u0315\u0316\u0005D\u0000\u0000\u0316\u0323\u0005"+
		"\u000b\u0000\u0000\u0317\u0318\u0005$\u0000\u0000\u0318\u0319\u0005\n"+
		"\u0000\u0000\u0319\u031a\u0005D\u0000\u0000\u031a\u0323\u0005\u000b\u0000"+
		"\u0000\u031b\u031c\u0005%\u0000\u0000\u031c\u031d\u0005\n\u0000\u0000"+
		"\u031d\u031e\u0005D\u0000\u0000\u031e\u0323\u0005\u000b\u0000\u0000\u031f"+
		"\u0323\u0005D\u0000\u0000\u0320\u0323\u0003d2\u0000\u0321\u0323\u0003"+
		">\u001f\u0000\u0322\u02f5\u0001\u0000\u0000\u0000\u0322\u02f6\u0001\u0000"+
		"\u0000\u0000\u0322\u02fb\u0001\u0000\u0000\u0000\u0322\u0300\u0001\u0000"+
		"\u0000\u0000\u0322\u0305\u0001\u0000\u0000\u0000\u0322\u030a\u0001\u0000"+
		"\u0000\u0000\u0322\u030f\u0001\u0000\u0000\u0000\u0322\u0313\u0001\u0000"+
		"\u0000\u0000\u0322\u0317\u0001\u0000\u0000\u0000\u0322\u031b\u0001\u0000"+
		"\u0000\u0000\u0322\u031f\u0001\u0000\u0000\u0000\u0322\u0320\u0001\u0000"+
		"\u0000\u0000\u0322\u0321\u0001\u0000\u0000\u0000\u0323c\u0001\u0000\u0000"+
		"\u0000\u0324\u0325\u0007\u0003\u0000\u0000\u0325e\u0001\u0000\u0000\u0000"+
		"\u0326\u0327\u00063\uffff\uffff\u0000\u0327\u0328\u0005\n\u0000\u0000"+
		"\u0328\u0329\u0003f3\u0000\u0329\u032a\u0003h4\u0000\u032a\u032b\u0003"+
		"f3\u0000\u032b\u032c\u0005\u000b\u0000\u0000\u032c\u0340\u0001\u0000\u0000"+
		"\u0000\u032d\u032e\u0003j5\u0000\u032e\u032f\u0005\n\u0000\u0000\u032f"+
		"\u0330\u0003f3\u0000\u0330\u0331\u0005\u000b\u0000\u0000\u0331\u0340\u0001"+
		"\u0000\u0000\u0000\u0332\u0333\u0005\n\u0000\u0000\u0333\u0334\u0003j"+
		"5\u0000\u0334\u0335\u0003f3\u0000\u0335\u0336\u0005\u000b\u0000\u0000"+
		"\u0336\u0340\u0001\u0000\u0000\u0000\u0337\u0338\u0005\n\u0000\u0000\u0338"+
		"\u0339\u0003f3\u0000\u0339\u033a\u0005\u000b\u0000\u0000\u033a\u0340\u0001"+
		"\u0000\u0000\u0000\u033b\u033c\u0003j5\u0000\u033c\u033d\u0003f3\u0002"+
		"\u033d\u0340\u0001\u0000\u0000\u0000\u033e\u0340\u0003b1\u0000\u033f\u0326"+
		"\u0001\u0000\u0000\u0000\u033f\u032d\u0001\u0000\u0000\u0000\u033f\u0332"+
		"\u0001\u0000\u0000\u0000\u033f\u0337\u0001\u0000\u0000\u0000\u033f\u033b"+
		"\u0001\u0000\u0000\u0000\u033f\u033e\u0001\u0000\u0000\u0000\u0340\u0347"+
		"\u0001\u0000\u0000\u0000\u0341\u0342\n\u0007\u0000\u0000\u0342\u0343\u0003"+
		"h4\u0000\u0343\u0344\u0003f3\b\u0344\u0346\u0001\u0000\u0000\u0000\u0345"+
		"\u0341\u0001\u0000\u0000\u0000\u0346\u0349\u0001\u0000\u0000\u0000\u0347"+
		"\u0345\u0001\u0000\u0000\u0000\u0347\u0348\u0001\u0000\u0000\u0000\u0348"+
		"g\u0001\u0000\u0000\u0000\u0349\u0347\u0001\u0000\u0000\u0000\u034a\u034b"+
		"\u0007\u0004\u0000\u0000\u034bi\u0001\u0000\u0000\u0000\u034c\u034d\u0007"+
		"\u0005\u0000\u0000\u034dk\u0001\u0000\u0000\u0000\u034e\u034f\u0005\u001c"+
		"\u0000\u0000\u034f\u0350\u0005\n\u0000\u0000\u0350\u0351\u0003`0\u0000"+
		"\u0351\u0352\u0005\u0003\u0000\u0000\u0352\u0353\u0003`0\u0000\u0353\u0354"+
		"\u0005\u0003\u0000\u0000\u0354\u0355\u0003b1\u0000\u0355\u0356\u0005\u000b"+
		"\u0000\u0000\u0356\u0357\u0005@\u0000\u0000\u0357m\u0001\u0000\u0000\u0000"+
		"Oq\u0080\u008d\u0094\u00a1\u00a7\u00b6\u00be\u00c4\u00ca\u00cf\u00d7\u00df"+
		"\u00e5\u00eb\u00f7\u00ff\u0105\u010b\u0116\u011e\u0124\u012a\u0135\u013d"+
		"\u0143\u0149\u0154\u015c\u0162\u0168\u016e\u0175\u017c\u0190\u0199\u01a0"+
		"\u01a5\u01aa\u01b5\u01bb\u01c9\u0206\u020b\u0214\u0216\u021f\u0223\u0234"+
		"\u023c\u0243\u024d\u0256\u025d\u0262\u0267\u0276\u027a\u0283\u028b\u0291"+
		"\u0297\u02a0\u02a8\u02af\u02b4\u02ba\u02c1\u02c6\u02d3\u02d6\u02de\u02e4"+
		"\u02e6\u02ee\u02f3\u0322\u033f\u0347";
	public static final ATN _ATN =
		new ATNDeserializer().deserialize(_serializedATN.toCharArray());
	static {
		_decisionToDFA = new DFA[_ATN.getNumberOfDecisions()];
		for (int i = 0; i < _ATN.getNumberOfDecisions(); i++) {
			_decisionToDFA[i] = new DFA(_ATN.getDecisionState(i), i);
		}
	}
}