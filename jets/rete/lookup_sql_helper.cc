
#include "sqlite3.h"

#include "../rdf/rdf_types.h"
#include "../rete/lookup_sql_helper.h"
#include "../rete/rete_session.h"


namespace jets::rete {

  int LookupTable::lookup_internal(ReteSession * rete_session, bool is_multi, std::string const& key, RDFTTYPE * out)
  {
    // min of validation
    if(not rete_session or not out) return -1;
    auto * rdf_session = rete_session->rdf_session();
    auto * rmgr = rdf_session->rmgr();

    // make the subject resource (lookup result associated to key)
    rdf::r_index subject = rmgr->create_resource(this->subject_prefix_+key);
    *out = *subject;
    auto lookup_row = rmgr->jets()->jets__lookup_row;
    auto lookup_multi_rows = rmgr->jets()->jets__lookup_multi_rows;

    // Check if the result of the lookup was already put in the rdf_session by a previous call
    if(rdf_session->contains(this->cache_uri_, lookup_row, subject)) {
      VLOG(3)<<"LOOKUP "<<this->lookup_name_<<" | KEY | "<<key<<" (CACHED)";
      return 0;
    }
      VLOG(3)<<"LOOKUP "<<this->lookup_name_<<" | KEY | "<<key;

    // Get the db connection and bind it to the key
    auto lc = this->db_pool_.get_connection();
    int err = sqlite3_reset(lc.stmt);
    if( err != SQLITE_OK ) return err;

    err = sqlite3_bind_text(lc.stmt, 1, key.c_str(), key.size(), nullptr);
    if( err != SQLITE_OK ) return err;

    try {      

      // Pull the result from the lookup table
      bool is_done = false;
      int count = 0;

      while(not is_done) {
        auto row_subject = is_multi ? rmgr->create_bnode():subject;

        // Get the row
        err = sqlite3_step( lc.stmt );
        if ( err == SQLITE_DONE ) {
          is_done = true;
          continue;
        }
        if(err != SQLITE_ROW) {
          LOG(ERROR) << "LookupTable::lookup: " <<
            "SQL error while reading lookup table '"<<this->lookup_name_<<"': " << err;
          *out = {};
          return err;
        }

        // Get the data out of the row
        rdf::r_index value;
        char* v = nullptr;
        int sz = boost::numeric_cast<int>(this->columns_.size());
        for(int pos=0; pos<sz; pos++) {
          ColumnInfo const& cinfo = this->columns_[pos];
          switch (cinfo.second) {
          case rdf::rdf_null_t             : value = rmgr->get_null(); break;
          case rdf::rdf_blank_node_t       : LOG(ERROR)<<"lookup: BUG: Blank node cannot be a returned lookup data type"; return -1;
          case rdf::rdf_literal_int32_t    : value = rmgr->create_literal(sqlite3_column_int(lc.stmt, pos)); break;
          case rdf::rdf_literal_uint32_t   : value = rmgr->create_literal(boost::numeric_cast<int>(sqlite3_column_int(lc.stmt, pos))); break;
          case rdf::rdf_literal_int64_t    : value = rmgr->create_literal(boost::numeric_cast<int64_t>(sqlite3_column_int64(lc.stmt, pos))); break;
          case rdf::rdf_literal_uint64_t   : value = rmgr->create_literal(boost::numeric_cast<uint64_t>(sqlite3_column_int64(lc.stmt, pos))); break;
          case rdf::rdf_literal_double_t   : value = rmgr->create_literal(sqlite3_column_double(lc.stmt, pos)); break;
          case rdf::rdf_named_resource_t   : 
          case rdf::rdf_literal_string_t   : 
          case rdf::rdf_literal_date_t     : 
          case rdf::rdf_literal_datetime_t : 
            v = (char*)sqlite3_column_text(lc.stmt, pos);
            if(v) {
              switch(cinfo.second) {
              case rdf::rdf_literal_date_t     : value = rmgr->create_literal(rdf::parse_date(v)); break;
              case rdf::rdf_literal_datetime_t : value = rmgr->create_literal(rdf::parse_datetime(v)); break;
              case rdf::rdf_named_resource_t   : value = rmgr->create_resource(v); break;
              default:
                value = rmgr->create_literal(v);
              }
            } else {
              value = rmgr->get_null();
            }
            break;
          default: {
            LOG(ERROR)<<"lookup: BUG: which type is out of range: "<<cinfo.second;
            return -1;
          }
          }
          rdf_session->insert_inferred(row_subject, cinfo.first, value);
        }
        if(is_multi) {
          rdf_session->insert_inferred(subject, lookup_multi_rows, row_subject);
        } else {
          is_done = true;
        }
        count += 1;
      }
      if(not count) {
        // got no row, return null
        *out = {};
        this->db_pool_.put_connection(lc);
        return 0;
      }
      rdf_session->insert_inferred(this->cache_uri_, lookup_row, subject);
    } catch(rete_exception ex) {
      LOG(ERROR)<<"lookup_sql_helper::lookup: ERROR Got Exception: "<<ex;
      this->db_pool_.put_connection(lc);
      return -1;
    } catch(...) {
      LOG(ERROR)<<"lookup_sql_helper::lookup: ERROR Got Unknown Exception!";
      this->db_pool_.put_connection(lc);
      return -1;
    }
    this->db_pool_.put_connection(lc);
    return 0;
  }

} // namespace jets::rete
