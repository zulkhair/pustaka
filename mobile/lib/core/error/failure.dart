/// Typed failures surfaced by the data layer. Sealed so callers can switch
/// exhaustively.
sealed class Failure implements Exception {
  const Failure(this.message);
  final String message;

  @override
  String toString() => '$runtimeType: $message';
}

/// Transport-level problem (no/again connection, timeout, non-HTTP error).
class NetworkFailure extends Failure {
  const NetworkFailure([super.message = 'network error']);
}

/// Backend returned the envelope with status != 0.
class ApiFailure extends Failure {
  const ApiFailure(this.status, String message) : super(message);
  final int status;
}

/// Authentication lost (refresh failed / 401 after retry).
class AuthFailure extends Failure {
  const AuthFailure([super.message = 'unauthorized']);
}

/// Client-side validation problem.
class ValidationFailure extends Failure {
  const ValidationFailure([super.message = 'validation failed']);
}

/// Anything we did not anticipate.
class UnknownFailure extends Failure {
  const UnknownFailure([super.message = 'unknown error']);
}
