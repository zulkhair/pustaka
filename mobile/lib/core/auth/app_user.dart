class AppUser {
  const AppUser({
    required this.id,
    required this.username,
    required this.email,
    required this.role,
    required this.emailVerified,
  });

  final String id;
  final String username;
  final String email;
  final String role;
  final bool emailVerified;

  factory AppUser.fromJson(Map<String, dynamic> json) {
    return AppUser(
      id: json['id'] as String,
      username: json['username'] as String,
      email: json['email'] as String,
      role: json['role'] as String? ?? 'user',
      emailVerified: json['emailVerified'] as bool? ?? false,
    );
  }
}
