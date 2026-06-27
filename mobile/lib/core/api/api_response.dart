/// The backend's uniform response envelope: `{status, message, data}`.
/// `status == 0` is success, `status == 1` is error.
class ApiResponse<T> {
  const ApiResponse({required this.status, required this.message, this.data});

  final int status;
  final String message;
  final T? data;

  bool get isOk => status == 0;

  factory ApiResponse.fromJson(
    Map<String, dynamic> json,
    T Function(Object? data) parse,
  ) {
    return ApiResponse(
      status: json['status'] as int? ?? 1,
      message: json['message'] as String? ?? '',
      data: json['data'] == null ? null : parse(json['data']),
    );
  }
}
