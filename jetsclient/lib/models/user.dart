/// The [UserModel] represent the logged-in user
class UserModel {
  /// The user name obtained from the token
  String? name;

  /// The login email
  String? email;

  /// The jwt token (obtained by login form)
  String? token;

  /// The user's password (used in login form)
  String? password;

  /// The user's password (used in login form)
  bool isAdmin;

  UserModel({this.name, this.email, this.token, this.password, this.isAdmin=false});

  bool get isAuthenticated => token != null && token!.isNotEmpty;
}
