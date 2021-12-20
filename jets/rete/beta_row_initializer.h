#ifndef JETS_RETE_BETA_ROW_INITIALIZER_H
#define JETS_RETE_BETA_ROW_INITIALIZER_H

#include <memory>

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
  using const_iterator = int const*;


  BetaRowInitializer() 
    : data_(nullptr),
      size_(0)
  {}

  explicit BetaRowInitializer(int size) 
    : data_(nullptr),
      size_(size)
  {
    if(size_ > 0) data_ = new int[size_ + 1]; // +1 for end()
  }

  virtual inline
  ~BetaRowInitializer()
  {
    if(data_) delete [] data_;
  }

  BetaRowInitializer(BetaRowInitializer const&) = delete;
  BetaRowInitializer & operator=(BetaRowInitializer const&) = delete;

  inline int
  get_size()const
  {
    return size_;
  }

  // Method to initialize data_ 
  // return -1 if called with invalid pos
  inline int 
  put(int pos, int val)
  {
    if(pos < 0 or pos >= size_) return -1;
    data_[pos] = val;
    return 0;
  }

  // Method to get data_[pos] 
  // return -1 if called with invalid pos
  inline int 
  get(int pos)const
  {
    if(pos < 0 or pos >= size_) return -1;
    return data_[pos];
  }

  // const_iterator used to initialize BetaRow upon row creation
  inline const_iterator
  begin()const
  {
    if(data_) return &data_[0];
    return nullptr;
  }

  inline const_iterator
  end()const
  {
    if(data_) return &data_[size_];
    return nullptr;
  }

 protected:

 private:
  // friend class find_visitor<RDFGraph>;
  // friend class RDFSession<RDFGraph>;

  int *  data_;
  int    size_;
};

inline 
BetaRowInitializerPtr create_row_initializer(int size)
{
  return std::make_shared<BetaRowInitializer>(size);
}

} // namespace jets::rete
#endif // JETS_RETE_BETA_ROW_INITIALIZER_H
