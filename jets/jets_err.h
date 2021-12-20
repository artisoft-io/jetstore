#ifndef JETS_ERRORS_H
#define JETS_ERRORS_H

#include <exception>
#include <iostream>
#include <sstream>
#include <string>

namespace jets {
/**
 * Main exception class for jets.
 */
class jets_exception : virtual public std::exception {
public:
  inline jets_exception() throw()
      : std::exception(), m_message("generic jets_exception"), m_str("") {}

  inline jets_exception(char const *message_) throw()
      : std::exception(), m_message(message_), m_str("") {}

  inline jets_exception(std::string const &message) throw()
      : std::exception(), m_message(nullptr), m_str(message) {}

  ~jets_exception() throw() {}

  inline char const *what() const throw() {
    return m_message ? m_message : m_str.c_str();
  }

private:
  friend std::ostream &operator<<(std::ostream &out, jets_exception const &r);

  char const *m_message;
  std::string m_str;
};

inline std::ostream &operator<<(std::ostream &out, jets_exception const &e) {
  return out << e.what();
}

inline std::string to_string(jets_exception const &e) {
  std::ostringstream streamOut;
  streamOut << e;
  return streamOut.str();
}
} // namespace jets
#define JETS_EXCEPTION(message)                                                \
  {                                                                            \
    std::ostringstream streamOut;                                              \
    streamOut << message;                                                      \
    throw jets::jets_exception(streamOut.str());                               \
  }

#endif // JETS_ERRORS_H
