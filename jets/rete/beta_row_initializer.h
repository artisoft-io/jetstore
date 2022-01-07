#ifndef JETS_RETE_BETA_ROW_INITIALIZER_H
#define JETS_RETE_BETA_ROW_INITIALIZER_H

#include <cstddef>
#include <memory>
#include <string>
#include <string_view>
#include <vector>

// Metadata Component describing the schema of BetaRow::data
namespace jets::rete {
// //////////////////////////////////////////////////////////////////////////////////////
// BetaRowInitializer class -- Component to initialize BetaRow when created while adding
// a row to the BetaRelation
// --------------------------------------------------------------------------------------
// enum to encode in a column position (int) if it is a position in the LHS parent node vertex row or in the RHS triple
enum ColumnPositionEncoding {
    brc_parent_node    = 0x1000,
    brc_triple         = 0x2000,
    brc_hi_mask        = 0xF000,
    brc_low_mask       = 0x0FFF,
};

class BetaRowInitializer;
using BetaRowInitializerPtr = std::shared_ptr<BetaRowInitializer>;

// BetaRelation making the rete network
class BetaRowInitializer {
 public:
  using data_vector = std::vector<int>;
  using label_vector = std::vector<std::string>;
  using data_const_iterator = data_vector::const_iterator;
  using labels_const_iterator = label_vector::const_iterator;


  BetaRowInitializer() 
    : data_(),
      labels_()
  {}

  explicit BetaRowInitializer(int size) 
    : data_(size),
      labels_(size)
  {}

  virtual inline
  ~BetaRowInitializer()
  {}

  BetaRowInitializer(BetaRowInitializer const&) = delete;
  BetaRowInitializer & operator=(BetaRowInitializer const&) = delete;

  inline int
  get_size()const
  {
    return (int)data_.size();
  }

  // Method to initialize data_ 
  // return -1 if called with invalid pos
  inline int 
  put(int pos, int val, std::string_view label)
  {
    if(pos < 0 or pos >= this->get_size()) return -1;
    data_[pos] = val;
    labels_[pos] = label;
    return 0;
  }

  // Method to get data_[pos] 
  // return -1 if called with invalid pos
  inline int 
  get(int pos)const
  {
    if(pos < 0 or pos >= this->get_size()) return -1;
    return data_[pos];
  }

  // Method to get labels_[pos] 
  // return empty if called with invalid pos
  inline std::string_view
  get_label(int pos)const
  {
    if(pos < 0 or pos >= this->get_size()) return {};
    return labels_[pos];
  }

  // const_iterator used to initialize BetaRow upon row creation
  inline data_const_iterator
  begin()const
  {
    return data_.begin();
  }

  inline data_const_iterator
  end()const
  {
    return data_.end();
  }

  inline labels_const_iterator
  labels_begin()const
  {
    return labels_.begin();
  }

  inline labels_const_iterator
  labels_end()const
  {
    return labels_.end();
  }

 protected:

 private:
  data_vector   data_;
  label_vector  labels_;
};

inline 
BetaRowInitializerPtr create_row_initializer(int size)
{
  return std::make_shared<BetaRowInitializer>(size);
}

} // namespace jets::rete
#endif // JETS_RETE_BETA_ROW_INITIALIZER_H
