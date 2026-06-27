class Tokens {
  const Tokens(
      {required this.access, required this.refresh, required this.expiresIn});

  final String access;
  final String refresh;
  final int expiresIn;

  factory Tokens.fromJson(Map<String, dynamic> json) {
    return Tokens(
      access: json['accessToken'] as String,
      refresh: json['refreshToken'] as String,
      expiresIn: json['expiresIn'] as int? ?? 0,
    );
  }
}
