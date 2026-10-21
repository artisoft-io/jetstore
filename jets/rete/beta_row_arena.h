#ifndef JETS_RETE_BETA_ROW_ARENA_H
#define JETS_RETE_BETA_ROW_ARENA_H

#include <cstddef>
#include <cstdint>
#include <new>
#include <type_traits>

#if defined(__SANITIZE_ADDRESS__)
#include <sanitizer/asan_interface.h>
#define JETS_ARENA_POISON(p, n) ASAN_POISON_MEMORY_REGION((p), (n))
#define JETS_ARENA_UNPOISON(p, n) ASAN_UNPOISON_MEMORY_REGION((p), (n))
#else
#define JETS_ARENA_POISON(p, n) ((void)(p), (void)(n))
#define JETS_ARENA_UNPOISON(p, n) ((void)(p), (void)(n))
#endif

// Memory owned by one ReteSession: its beta relations, their rows, and the nodes and
// bucket arrays of the containers that hold and index them.
//
// Why: a large session holds tens of millions of rows and container nodes, and freeing
// them one by one when the session is deleted cost a quarter of the session's time.
// Everything a session's beta relations own is allocated here, so the session is torn
// down by releasing the arena's blocks: the relations' destructors are not run (see
// ReteSession::~ReteSession).
//
// The arena is not a monotonic buffer. A row erased while the session runs, and a
// candidate row a filter rejects, is freed as before and its memory is reused: small
// blocks go back to a free list per 16-byte size class, large ones (the containers'
// bucket arrays) to the system. So a session's peak memory is what it was with the
// global heap, plus the unused tail of the current block.
//
// Not thread safe: a ReteSession is used by one thread at a time.
namespace jets::rete {

class BetaRowArena {
 public:
  static constexpr std::size_t kAlign = 16;
  static constexpr std::size_t kMaxSmall = 512;
  static constexpr std::size_t kFirstBlock = 16 * 1024;
  // Below glibc's initial mmap threshold (128 KiB), so a block never costs an mmap
  static constexpr std::size_t kMaxBlock = 64 * 1024;

  BetaRowArena() = default;
  ~BetaRowArena() { release(); }

  BetaRowArena(BetaRowArena const&) = delete;
  BetaRowArena & operator=(BetaRowArena const&) = delete;

  inline void *
  allocate(std::size_t bytes, std::size_t align = alignof(std::max_align_t))
  {
    if(align > kAlign) throw std::bad_alloc();
    if(bytes > kMaxSmall) return allocate_large(bytes);
    std::size_t const cls = size_class(bytes);
    std::size_t const sz = cls * kAlign;
    FreeNode * node = free_[cls];
    if(node) {
      JETS_ARENA_UNPOISON(node, sz);
      free_[cls] = node->next;
      ++live_small_;
      return node;
    }
    if(static_cast<std::size_t>(end_ - cur_) < sz) new_block();
    void * p = cur_;
    cur_ += sz;
    JETS_ARENA_UNPOISON(p, sz);
    ++live_small_;
    return p;
  }

  inline void
  deallocate(void * p, std::size_t bytes) noexcept
  {
    if(not p) return;
    if(bytes > kMaxSmall) {
      deallocate_large(p);
      return;
    }
    std::size_t const cls = size_class(bytes);
    auto * node = static_cast<FreeNode*>(p);
    node->next = free_[cls];
    free_[cls] = node;
    JETS_ARENA_POISON(p, cls * kAlign);
    --live_small_;
  }

  // Frees every block and every large allocation. Whatever was constructed in the
  // arena and not destroyed is gone: call it only when nothing refers to that memory.
  inline void
  release() noexcept
  {
    while(blocks_) {
      Block * next = blocks_->next;
      JETS_ARENA_UNPOISON(blocks_, blocks_->size);
      ::operator delete(static_cast<void*>(blocks_));
      blocks_ = next;
    }
    while(large_) {
      Large * next = large_->next;
      ::operator delete(static_cast<void*>(large_));
      large_ = next;
    }
    for(auto & f: free_) f = nullptr;
    cur_ = end_ = nullptr;
    next_block_size_ = kFirstBlock;
    block_bytes_ = 0;
    nbr_blocks_ = 0;
    large_bytes_ = 0;
    nbr_large_ = 0;
    live_small_ = 0;
  }

  // Counters, for tests and for reporting memory
  inline std::size_t block_bytes()const { return block_bytes_; }
  inline std::size_t nbr_blocks()const { return nbr_blocks_; }
  inline std::size_t large_bytes()const { return large_bytes_; }
  inline std::size_t nbr_large()const { return nbr_large_; }
  // Small allocations made and not yet deallocated
  inline std::size_t live_small()const { return live_small_; }

 private:
  struct FreeNode { FreeNode * next; };
  struct alignas(kAlign) Block { Block * next; std::size_t size; };
  struct alignas(kAlign) Large { Large * prev; Large * next; std::size_t size; };
  static_assert(sizeof(Block) % kAlign == 0);
  static_assert(sizeof(Large) % kAlign == 0);

  static inline std::size_t
  size_class(std::size_t bytes)
  {
    return bytes == 0 ? 1 : (bytes + kAlign - 1) / kAlign;
  }

  void
  new_block()
  {
    std::size_t const size = next_block_size_;
    auto * b = static_cast<Block*>(::operator new(size));
    b->next = blocks_;
    b->size = size;
    blocks_ = b;
    cur_ = reinterpret_cast<char*>(b) + sizeof(Block);
    end_ = reinterpret_cast<char*>(b) + size;
    JETS_ARENA_POISON(cur_, end_ - cur_);
    block_bytes_ += size;
    ++nbr_blocks_;
    if(next_block_size_ < kMaxBlock) next_block_size_ *= 2;
  }

  void *
  allocate_large(std::size_t bytes)
  {
    auto * h = static_cast<Large*>(::operator new(sizeof(Large) + bytes));
    h->prev = nullptr;
    h->next = large_;
    h->size = bytes;
    if(large_) large_->prev = h;
    large_ = h;
    large_bytes_ += bytes;
    ++nbr_large_;
    return h + 1;
  }

  void
  deallocate_large(void * p) noexcept
  {
    Large * h = static_cast<Large*>(p) - 1;
    if(h->prev) h->prev->next = h->next;
    else large_ = h->next;
    if(h->next) h->next->prev = h->prev;
    large_bytes_ -= h->size;
    --nbr_large_;
    ::operator delete(static_cast<void*>(h));
  }

  FreeNode *  free_[kMaxSmall / kAlign + 1] = {};
  char *      cur_ = nullptr;
  char *      end_ = nullptr;
  Block *     blocks_ = nullptr;
  Large *     large_ = nullptr;
  std::size_t next_block_size_ = kFirstBlock;
  std::size_t block_bytes_ = 0;
  std::size_t nbr_blocks_ = 0;
  std::size_t large_bytes_ = 0;
  std::size_t nbr_large_ = 0;
  std::size_t live_small_ = 0;
};

// Standard allocator over a BetaRowArena. A default-constructed one has no arena and
// uses the global heap, for the rows, relations and containers made outside a
// ReteSession (tests, temporaries).
template<class T>
class ArenaAllocator {
 public:
  using value_type = T;
  using propagate_on_container_copy_assignment = std::true_type;
  using propagate_on_container_move_assignment = std::true_type;
  using propagate_on_container_swap = std::true_type;
  using is_always_equal = std::false_type;
  static_assert(alignof(T) <= BetaRowArena::kAlign);

  ArenaAllocator() noexcept : arena_(nullptr) {}
  explicit ArenaAllocator(BetaRowArena * arena) noexcept : arena_(arena) {}
  template<class U>
  ArenaAllocator(ArenaAllocator<U> const& other) noexcept : arena_(other.arena()) {}

  inline T *
  allocate(std::size_t n)
  {
    if(arena_) return static_cast<T*>(arena_->allocate(n * sizeof(T), alignof(T)));
    return static_cast<T*>(::operator new(n * sizeof(T)));
  }

  inline void
  deallocate(T * p, std::size_t n) noexcept
  {
    if(arena_) arena_->deallocate(p, n * sizeof(T));
    else ::operator delete(static_cast<void*>(p));
  }

  inline BetaRowArena * arena()const noexcept { return arena_; }

 private:
  BetaRowArena * arena_;
};

template<class T, class U>
inline bool operator==(ArenaAllocator<T> const& a, ArenaAllocator<U> const& b) noexcept
{
  return a.arena() == b.arena();
}

} // namespace jets::rete
#endif // JETS_RETE_BETA_ROW_ARENA_H
