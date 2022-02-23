#ifndef JETS_RETE_META_STORE_FACTORY_HELPER_H
#define JETS_RETE_META_STORE_FACTORY_HELPER_H

#include <exception>
#include <iostream>
#include <memory>
#include <sstream>
#include <string>

#include "rete_err.h"
#include "rete_types.h"
#include "alpha_functors.h"

namespace jets::rete {

  enum FuncEnum {
    f_cst_e = 0,
    f_binded_e = 1,
    f_var_e = 2,
    f_expr_e = 3
  };

  class FuncFactory
  {
    public:
    FuncFactory() {}

    virtual
    ~FuncFactory() {}

    virtual
    int get_func_type()=0;

    virtual
    F_binded get_f_binded()
    {
      RETE_EXCEPTION("ERROR FuncFactory is not binded");
    }

    virtual
    F_cst get_f_cst()
    {
      RETE_EXCEPTION("ERROR FuncFactory is not const resource");
    }

    virtual
    F_var get_f_var()
    {
      RETE_EXCEPTION("ERROR FuncFactory is not a unbinded var");
    }

    virtual
    F_expr get_f_expr()
    {
      RETE_EXCEPTION("ERROR FuncFactory is not an expr");
    }
  };
  using FuncFactoryPtr = std::shared_ptr<FuncFactory>;

  class FcstFunc: public FuncFactory
  {
    public:
    FcstFunc(rdf::r_index r): FuncFactory(), r(r) {}

    virtual
    ~FcstFunc() {}

    int get_func_type() override
    {
      return f_cst_e;
    }

    F_cst get_f_cst() override
    {
      return F_cst(this->r);
    }
    rdf::r_index r;
  };

  class FbindedFunc: public FuncFactory
  {
    public:
    FbindedFunc(int pos): FuncFactory(), pos(pos) {}

    virtual
    ~FbindedFunc() {}

    int get_func_type() override
    {
      return f_binded_e;
    }

    F_binded get_f_binded() override
    {
      return F_binded(this->pos);
    }
    int pos;
  };

  class FvarFunc: public FuncFactory
  {
    public:
    FvarFunc(std::string id): FuncFactory(), id(id) {}

    virtual
    ~FvarFunc() {}

    int get_func_type() override
    {
      return f_var_e;
    }

    F_var get_f_var() override
    {
      return F_var(this->id);
    }
    std::string id;
  };

  class FexprFunc: public FuncFactory
  {
    public:
    FexprFunc(ExprBasePtr expr): FuncFactory(), expr(expr) {}

    virtual
    ~FexprFunc() {}

    int get_func_type() override
    {
      return f_expr_e;
    }

    F_expr get_f_expr() override
    {
      return F_expr(this->expr);
    }
    ExprBasePtr expr;
  };

} // namespace jets::rete

#endif // JETS_RETE_META_STORE_FACTORY_HELPER_H
