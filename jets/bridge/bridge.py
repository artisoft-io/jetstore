import ctypes
import os
from enum import Enum

# path = os.path.abspath(os.getcwd())
# print('***BRIDGE.PY CURRENT DIR:',path)

class ResourceType(Enum):
  RDF_NULL             = 0 
  RDF_BLANK_NODE       = 1 
  RDF_NAMED_RESOURCE   = 2 
  RDF_LITERAL_INT32    = 3 
  RDF_LITERAL_UINT32   = 4 
  RDF_LITERAL_INT64    = 5 
  RDF_LITERAL_UINT64   = 6 
  RDF_LITERAL_DOUBLE   = 7 
  RDF_LITERAL_STRING   = 8 

# Load the shared library into ctypes
c_lib = ctypes.CDLL("libjets.so")

# ---------------------------------------------------------------------------------------
c_lib.create_jetstore_hdl.argtypes = [ctypes.c_char_p, ctypes.c_char_p, ctypes.c_void_p]
c_lib.create_jetstore_hdl.restype = ctypes.c_int

def createJetStoreHandle(rete_db_fname: str, lookup_data_db_fname: str) -> ctypes.c_void_p:
  if not rete_db_fname or not lookup_data_db_fname:
    return None

  if isinstance(rete_db_fname, str):
      rete_db_fname = rete_db_fname.encode('utf-8')
  elif not isinstance(rete_db_fname, bytes):
      raise Exception('createJetStoreHandle: Arguments must be str or bytes')

  if isinstance(lookup_data_db_fname, str):
      lookup_data_db_fname = lookup_data_db_fname.encode('utf-8')
  elif not isinstance(lookup_data_db_fname, bytes):
      raise Exception('createJetStoreHandle: Arguments must be str or bytes')

  js_hdlr = ctypes.c_void_p()
  res = c_lib.create_jetstore_hdl(rete_db_fname, lookup_data_db_fname, ctypes.byref(js_hdlr))
  if res:
    raise Exception('createJetStoreHandle: ERROR: '+str(res))
  return js_hdlr

# ---------------------------------------------------------------------------------------
c_lib.delete_jetstore_hdl.argtypes = [ctypes.c_void_p]
c_lib.delete_jetstore_hdl.restype = ctypes.c_int

def deleteJetStoreHandle(js_hdlr: ctypes.c_void_p) -> None:
  if not js_hdlr:
    print('deleteJetStoreHandle: Handle already released!')
    return None

  res = c_lib.delete_jetstore_hdl(js_hdlr)
  if res:
    raise Exception('deleteJetStoreHandle: ERROR: '+str(res))
  return None

# ---------------------------------------------------------------------------------------
c_lib.create_rete_session.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.c_void_p]
c_lib.create_rete_session.restype = ctypes.c_int

def createReteSession(js_hdlr: ctypes.c_void_p, txt: str) -> ctypes.c_void_p:
  if not txt:
    raise Exception('createReteSession: Must provide rule name')

  if isinstance(txt, str):
      txt = txt.encode('utf-8')
  elif not isinstance(txt, bytes):
      raise Exception('createReteSession: Argument must be str or bytes')

  rs_hdlr = ctypes.c_void_p()
  res = c_lib.create_rete_session(js_hdlr, txt, ctypes.byref(rs_hdlr))
  if res:
    raise Exception('createReteSession: ERROR: '+str(res))
  return rs_hdlr

# ---------------------------------------------------------------------------------------
c_lib.delete_rete_session.argtypes = [ctypes.c_void_p]
c_lib.delete_rete_session.restype = ctypes.c_int

def deleteReteSession(rs_hdlr: ctypes.c_void_p) -> None:
  if not rs_hdlr:
    print('deleteReteSession: Handle already released!')
    return None

  res = c_lib.delete_rete_session(rs_hdlr)
  if res:
    raise Exception('deleteReteSession: ERROR: '+str(res))
  return None

# ---------------------------------------------------------------------------------------
c_lib.create_resource.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.c_void_p]
c_lib.create_resource.restype = ctypes.c_int

def createResource(rete_session_hdlr: ctypes.c_void_p, txt: str) -> ctypes.c_void_p:
  if not rete_session_hdlr:
    raise Exception('createResource: ERROR must provide valid ReteSession handle: '+str(res))
  if not txt:
    raise Exception('createResource: Must provide resource name')

  if isinstance(txt, str):
      txt = txt.encode('utf-8')
  elif not isinstance(txt, bytes):
      raise Exception('createResource: Argument must be str or bytes')

  r_hdlr = ctypes.c_void_p()
  res = c_lib.create_resource(rete_session_hdlr, txt, ctypes.byref(r_hdlr))
  if res:
    raise Exception('createResource: ERROR: '+str(res))
  return r_hdlr

# ---------------------------------------------------------------------------------------
c_lib.create_text.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.c_void_p]
c_lib.create_text.restype = ctypes.c_int

def createText(rete_session_hdlr: ctypes.c_void_p, txt: str) -> ctypes.c_void_p:
  if not rete_session_hdlr:
    raise Exception('createText: ERROR must provide valid ReteSession handle: '+str(res))
  if not txt:
    raise Exception('createText: Must provide text')

  if isinstance(txt, str):
      txt = txt.encode('utf-8')
  elif not isinstance(txt, bytes):
      raise Exception('createText: Argument must be str or bytes')

  r_hdlr = ctypes.c_void_p()
  res = c_lib.create_text(rete_session_hdlr, txt, ctypes.byref(r_hdlr))
  if res:
    raise Exception('createText: ERROR: '+str(res))
  return r_hdlr

# ---------------------------------------------------------------------------------------
c_lib.create_int.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.c_void_p]
c_lib.create_int.restype = ctypes.c_int

def createInt(rete_session_hdlr: ctypes.c_void_p, value: int) -> ctypes.c_void_p:
  if not rete_session_hdlr:
    raise Exception('createInt: ERROR must provide valid ReteSession handle: '+str(res))
  if value is None:
    raise Exception('createInt: Must provide value')

  if not isinstance(value, int):
      raise Exception('createInt: Argument must be int')

  r_hdlr = ctypes.c_void_p()
  res = c_lib.create_int(rete_session_hdlr, value, ctypes.byref(r_hdlr))
  if res:
    raise Exception('createInt: ERROR: '+str(res))
  return r_hdlr

# ---------------------------------------------------------------------------------------
c_lib.get_resource_type.argtypes = [ctypes.c_void_p]
c_lib.get_resource_type.restype = ctypes.c_int

def getResourceType(r_hdlr: ctypes.c_void_p) -> int:
  if not r_hdlr:
    raise Exception('getResourceType: Handle must not be null!')

  res: int = c_lib.get_resource_type(r_hdlr)
  if res<0:
    raise Exception('getResourceType: ERROR: '+str(res))
  return res

# ---------------------------------------------------------------------------------------
c_lib.get_resource_name.argtypes = [ctypes.c_void_p, ctypes.c_void_p]
c_lib.get_resource_name.restype = ctypes.c_int

def getResourceName(r_hdlr: ctypes.c_void_p) -> str:
  if not r_hdlr:
    raise Exception('getResourceName: Handle must not be null!')

  ss = ctypes.c_char_p()
  str_hdlr = ctypes.pointer(ss)
  res = c_lib.get_resource_name(r_hdlr, str_hdlr)
  if res:
    raise Exception('getResourceName: ERROR: '+str(res))
  return ss.value.decode()

# ---------------------------------------------------------------------------------------
c_lib.get_int_literal.argtypes = [ctypes.c_void_p, ctypes.c_int]
c_lib.get_int_literal.restype = ctypes.c_int

def getIntValue(r_hdlr: ctypes.c_void_p) -> int:
  if not r_hdlr:
    raise Exception('getIntValue: Handle must not be null!')

  value = ctypes.c_int()
  res = c_lib.get_int_literal(r_hdlr, ctypes.byref(value))
  if res:
    raise Exception('getIntValue: ERROR: '+str(res))
  return value

# ---------------------------------------------------------------------------------------
c_lib.get_text_literal.argtypes = [ctypes.c_void_p, ctypes.c_char_p]
c_lib.get_text_literal.restype = ctypes.c_int

def getTextValue(r_hdlr: ctypes.c_void_p) -> str:
  if not r_hdlr:
    raise Exception('getTextValue: Handle must not be null!')

  ss = ctypes.c_char_p()
  str_hdlr = ctypes.pointer(ss)
  res = c_lib.get_text_literal(r_hdlr, str_hdlr)
  if res:
    raise Exception('getTextValue: ERROR: '+str(res))
  return ss.value.decode()

# ---------------------------------------------------------------------------------------
c_lib.insert.argtypes = [ctypes.c_void_p, ctypes.c_void_p, ctypes.c_void_p, ctypes.c_void_p]
c_lib.insert.restype = ctypes.c_int

def insertTriple(rs_hdlr: ctypes.c_void_p, s: ctypes.c_void_p, p: ctypes.c_void_p, o: ctypes.c_void_p) -> int:
  if not rs_hdlr or not s or not p or not o:
    raise Exception('insertTriple: Handle must not be null!')

  res = c_lib.insert(rs_hdlr, s, p, o)
  if res < 0:
    raise Exception('insertTriple: ERROR: '+str(res))
  return res

# ---------------------------------------------------------------------------------------
c_lib.contains.argtypes = [ctypes.c_void_p, ctypes.c_void_p, ctypes.c_void_p, ctypes.c_void_p]
c_lib.contains.restype = ctypes.c_int

def containsTriple(rs_hdlr: ctypes.c_void_p, s: ctypes.c_void_p, p: ctypes.c_void_p, o: ctypes.c_void_p) -> bool:
  if not rs_hdlr or not s or not p or not o:
    raise Exception('containsTriple: Handle must not be null!')

  res = c_lib.contains(rs_hdlr, s, p, o)
  if res < 0:
    raise Exception('containsTriple: ERROR: '+str(res))
  return res

# ---------------------------------------------------------------------------------------
c_lib.execute_rules.argtypes = [ctypes.c_void_p]
c_lib.execute_rules.restype = ctypes.c_int

def executeRules(rs_hdlr: ctypes.c_void_p) -> None:
  if not rs_hdlr:
    raise Exception('executeRules: Handle must not be null!')

  res = c_lib.execute_rules(rs_hdlr)
  if res < 0:
    raise Exception('executeRules: ERROR: '+str(res))
  return None

# ---------------------------------------------------------------------------------------
c_lib.find_all.argtypes = [ctypes.c_void_p, ctypes.c_void_p]
c_lib.find_all.restype = ctypes.c_int

def findAll(rs_hdlr: ctypes.c_void_p) -> ctypes.c_void_p:
  if not rs_hdlr:
    raise Exception('findAll: Handle must not be null!')

  itor_hdlr = ctypes.c_void_p()
  res = c_lib.find_all(rs_hdlr, ctypes.byref(itor_hdlr))
  if res:
    raise Exception('findAll: ERROR: '+str(res))
  return itor_hdlr

# ---------------------------------------------------------------------------------------
c_lib.is_end.argtypes = [ctypes.c_void_p]
c_lib.is_end.restype = ctypes.c_int

def isEnd(itor_hdlr: ctypes.c_void_p) -> bool:
  if not itor_hdlr:
    raise Exception('isEnd: Handle must not be null!')

  res = c_lib.is_end(itor_hdlr)
  if res < 0:
    raise Exception('isEnd: ERROR: '+str(res))
  return res

# ---------------------------------------------------------------------------------------
c_lib.next.argtypes = [ctypes.c_void_p]
c_lib.next.restype = ctypes.c_int

def next(itor_hdlr: ctypes.c_void_p) -> bool:
  if not itor_hdlr:
    raise Exception('next: Handle must not be null!')

  res = c_lib.next(itor_hdlr)
  if res < 0:
    raise Exception('next: ERROR: '+str(res))
  return res

# ---------------------------------------------------------------------------------------
c_lib.get_subject.argtypes = [ctypes.c_void_p, ctypes.c_void_p]
c_lib.get_subject.restype = ctypes.c_int

def getSubject(itor_hdlr: ctypes.c_void_p) -> ctypes.c_void_p:
  if not itor_hdlr:
    raise Exception('getSubject: Handle must not be null!')

  r_hdlr = ctypes.c_void_p()
  res = c_lib.get_subject(itor_hdlr, ctypes.byref(r_hdlr))
  if res:
    raise Exception('getSubject: ERROR: '+str(res))
  return r_hdlr

# ---------------------------------------------------------------------------------------
c_lib.get_predicate.argtypes = [ctypes.c_void_p, ctypes.c_void_p]
c_lib.get_predicate.restype = ctypes.c_int

def getPredicate(itor_hdlr: ctypes.c_void_p) -> ctypes.c_void_p:
  if not itor_hdlr:
    raise Exception('getPredicate: Handle must not be null!')

  r_hdlr = ctypes.c_void_p()
  res = c_lib.get_predicate(itor_hdlr, ctypes.byref(r_hdlr))
  if res:
    raise Exception('getPredicate: ERROR: '+str(res))
  return r_hdlr

# ---------------------------------------------------------------------------------------
c_lib.get_object.argtypes = [ctypes.c_void_p, ctypes.c_void_p]
c_lib.get_object.restype = ctypes.c_int

def getObject(itor_hdlr: ctypes.c_void_p) -> ctypes.c_void_p:
  if not itor_hdlr:
    raise Exception('getObject: Handle must not be null!')

  r_hdlr = ctypes.c_void_p()
  res = c_lib.get_object(itor_hdlr, ctypes.byref(r_hdlr))
  if res:
    raise Exception('getObject: ERROR: '+str(res))
  return r_hdlr

# ---------------------------------------------------------------------------------------
c_lib.dispose.argtypes = [ctypes.c_void_p]
c_lib.dispose.restype = ctypes.c_int

def disposeIterator(itor_hdlr: ctypes.c_void_p) -> None:
  if not itor_hdlr:
    print('disposeIterator: Handle already released!')
    return None

  res = c_lib.dispose(itor_hdlr)
  if res:
    raise Exception('disposeIterator: ERROR: '+str(res))
  return None
