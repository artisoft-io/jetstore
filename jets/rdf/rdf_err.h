#ifndef JETS_RDF_ERRORS_H
#define JETS_RDF_ERRORS_H

#include <exception>
#include <iostream>
#include <sstream>
#include <string>

namespace jets::rdf {
/**
 * Main exception class for jets.
 */
class rdf_exception : virtual public std::exception {
public:
  inline rdf_exception() throw()
      : std::exception(), m_message("generic rdf_exception"), m_str("") {}

  inline rdf_exception(char const *message_) throw()
      : std::exception(), m_message(message_), m_str("") {}

  inline rdf_exception(std::string const &message) throw()
      : std::exception(), m_message(nullptr), m_str(message) {}

  ~rdf_exception() throw() {}

  inline char const *what() const throw() {
    return m_message ? m_message : m_str.c_str();
  }

private:
  friend std::ostream &operator<<(std::ostream &out, rdf_exception const &r);

  char const *m_message;
  std::string m_str;
};

inline std::ostream &operator<<(std::ostream &out, rdf_exception const &e) {
  return out << e.what();
}

inline std::string to_string(rdf_exception const &e) {
  std::ostringstream streamOut;
  streamOut << e;
  return streamOut.str();
}
} // namespace jets::rdf

#define RDF_EXCEPTION(message)                                                 \
  {                                                                            \
    std::ostringstream streamOut;                                              \
    streamOut << message;                                                      \
    throw jets::rdf::rdf_exception(streamOut.str());                           \
  }

#endif // JETS_RDF_ERRORS_H
