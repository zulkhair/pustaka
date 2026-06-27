/// A read-only document share (backend ShareDTO:
/// {userId, username, email, permission, createdAt}). The create response omits
/// username/email, so those may be empty until the list is re-fetched.
class DocumentShare {
  const DocumentShare({
    required this.userId,
    required this.email,
    required this.permission,
    required this.createdAt,
    this.username,
  });

  final String userId;
  final String email;
  final String permission;
  final DateTime createdAt;
  final String? username;

  factory DocumentShare.fromJson(Map<String, dynamic> json) {
    return DocumentShare(
      userId: json['userId'] as String? ?? '',
      email: json['email'] as String? ?? '',
      permission: json['permission'] as String? ?? 'viewer',
      username: json['username'] as String?,
      createdAt: DateTime.tryParse(json['createdAt'] as String? ?? '') ??
          DateTime.fromMillisecondsSinceEpoch(0),
    );
  }
}
