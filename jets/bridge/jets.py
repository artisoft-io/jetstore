"""Domain model for bridge.py"""

import ctypes
import bridge as api


class Resource:
  def __init__(self, r_hdl: ctypes.c_void_p) -> None:
    self.r_hdl = r_hdl
  
  # Get Resource Type
  def get_type(self) -> int:
    return api.getResourceType(self.r_hdl)
  
  # Get Resource Name
  def get_name(self) -> str:
    return api.getResourceName(self.r_hdl)
  
  # Get int from literal
  def get_int(self) -> int:
    return api.getIntValue(self.r_hdl)
  
  # Get text from literal
  def get_text(self) -> str:
    return api.getTextValue(self.r_hdl)

  # String representation of resource
  def __str__(self) -> str:
    tp = api.getResourceType(self.r_hdl)
    if tp == 0: return "NULL"
    if tp == 1: return "BN:"
    if tp == 2: return self.get_name()
    if tp == 3: return str(self.get_int())
    if tp == 8: return self.get_text()
    raise ValueError("ERROR string representation of Resource type",tp,"not implemented!")


class ReteSessionIterator:
  def __init__(self, rs_hdl: ctypes.c_void_p, itor_hdl: ctypes.c_void_p) -> None:
    self.rs_hdl = rs_hdl    # ReteSession handle
    self.itor_hdl = itor_hdl

  def is_end(self) -> bool:
    return api.isEnd(self.itor_hdl)

  def next(self) -> bool:
    return api.next(self.itor_hdl)

  def get_subject(self) -> Resource:
    r_hdl = api.getSubject(self.itor_hdl)
    return Resource(r_hdl)

  def get_predicate(self) -> Resource:
    r_hdl = api.getPredicate(self.itor_hdl)
    return Resource(r_hdl)

  def get_object(self) -> Resource:
    r_hdl = api.getObject(self.itor_hdl)
    return Resource(r_hdl)


class ReteSession:
  def __init__(self, rs_hdl: ctypes.c_void_p, jetrules_name: str) -> None:
    self.rs_hdl = rs_hdl
    self.jetrules_name = jetrules_name
  
  # Create Resource
  def create_resource(self, name: str):
    r_hdl = api.createResource(self.rs_hdl, name)
    return Resource(r_hdl)
  
  # Get Resource
  def create_resource(self, name: str):
    r_hdl = api.createResource(self.rs_hdl, name)
    return Resource(r_hdl)

  # Insert Triple
  def insert_triple(self, s: Resource, p: Resource, o: Resource) -> bool:
    ret = api.insertTriple(self.rs_hdl, s.r_hdl, p.r_hdl, o.r_hdl)
    return bool(ret)

  # Contains Triple
  def contains_triple(self, s: Resource, p: Resource, o: Resource) -> bool:
    ret = api.containsTriple(self.rs_hdl, s.r_hdl, p.r_hdl, o.r_hdl)
    return bool(ret)

  # Execute Rules
  def execute_rules(self) -> None:
    api.executeRules(self.rs_hdl)

  # Find All in ReteSession
  def find_all(self) -> ReteSessionIterator:
    itor_hdl = api.findAll(self.rs_hdl)
    return ReteSessionIterator(self.rs_hdl, itor_hdl)


class JetStoreFactory:
  def __init__(self, js_hdl: ctypes.c_void_p, rete_db_fname: str, lookup_data_db_fname: str) -> None:
    self.js_hdl = js_hdl
    self.rete_db_fname = rete_db_fname
    self.lookup_data_db_fname = lookup_data_db_fname

  # Start a rete session
  def create_rete_session(self, jetrules_name: str) -> ReteSession:
    rs_hdl = api.createReteSession(self.js_hdl, jetrules_name)
    return ReteSession(rs_hdl, jetrules_name)


# Create and Load JetStore Factory
def create_jetstore_factory(rete_db_fname: str, lookup_data_db_fname: str) -> JetStoreFactory:
  js_hdl = api.createJetStoreHandle(rete_db_fname, lookup_data_db_fname)
  return JetStoreFactory(js_hdl, rete_db_fname, lookup_data_db_fname)

