import ctypes
import os

path = os.path.abspath(os.getcwd())
print('CURRENT DIR:',path)

# if not os.path.exists(path) or not os.path.isfile(path):
#   print('ERROR: JetRule file {0} does not exist or is not a file'.format(path))

# Load the shared library into ctypes
c_lib = ctypes.CDLL("jets/rete/libjets_rete.so")

# ---------------------------------------------------------------------------------------
c_lib.create_jetstore_hdl.argtypes = [ctypes.c_char_p, ctypes.c_void_p]
c_lib.create_jetstore_hdl.restype = ctypes.c_int

def createJetStoreHandle(txt: str) -> ctypes.c_void_p:
  if not txt:
    return None

  if isinstance(txt, str):
      txt = txt.encode('utf-8')
  elif not isinstance(txt, bytes):
      raise Exception('createJetStoreHandle: Argument must be str or bytes')

  js_hdlr = ctypes.c_void_p()
  res = c_lib.create_jetstore_hdl(txt, ctypes.byref(js_hdlr))
  return js_hdlr

# ---------------------------------------------------------------------------------------
c_lib.delete_jetstore_hdl.argtypes = [ctypes.c_void_p]
c_lib.delete_jetstore_hdl.restype = ctypes.c_int

def deleteJetStoreHandle(js_hdlr: ctypes.c_void_p) -> None:
  if not js_hdlr:
    print('delete_jetstore_hdl: Handle already released!')
    return None

  res = c_lib.delete_jetstore_hdl(js_hdlr)
  if res:
    print('delete_jetstore_hdl: ERROR return from c:',res)
  return None

# ---------------------------------------------------------------------------------------
c_lib.create_rete_session.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.c_void_p]
c_lib.create_rete_session.restype = ctypes.c_int

def createReteSession(js_hdlr: ctypes.c_void_p, txt: str) -> ctypes.c_void_p:
  if not txt:
    print('createReteSession: Must provide rule name')
    return None

  if isinstance(txt, str):
      txt = txt.encode('utf-8')
  elif not isinstance(txt, bytes):
      raise Exception('createReteSession: Argument must be str or bytes')

  rs_hdlr = ctypes.c_void_p()
  res = c_lib.create_rete_session(js_hdlr, txt, ctypes.byref(rs_hdlr))
  print("OK SESSION RDY")
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
    print('delete_rete_session: ERROR return from c:',res)
  return None
