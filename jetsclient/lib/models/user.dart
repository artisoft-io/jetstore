import 'package:json_annotation/json_annotation.dart';
part 'user.g.dart';

/// The [UserModel] represent the logged-in user
@JsonSerializable()
class UserModel {
  /// The user name obtained from the token
  String? name;

  /// The login email
  String? email;

  /// The jwt token (obtained by login form)
  String? token;

  /// The user's password (used in login form)
  String? password;

  UserModel({this.name, this.email, this.token, this.password});

  bool get isAuthenticated => token != null && token!.isNotEmpty;

  factory UserModel.fromJson(Map<String, dynamic> json) =>
      _$UserModelFromJson(json);

  Map<String, dynamic> toJson() => _$UserModelToJson(this);
}
