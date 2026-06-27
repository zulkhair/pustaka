import 'package:flutter_test/flutter_test.dart';
import 'package:pustaka/core/api/api_response.dart';

void main() {
  test('parses data and isOk when status 0', () {
    final r = ApiResponse<Map<String, dynamic>>.fromJson(
      {
        'status': 0,
        'message': 'ok',
        'data': {'x': 1}
      },
      (d) => d as Map<String, dynamic>,
    );
    expect(r.isOk, isTrue);
    expect(r.message, 'ok');
    expect(r.data!['x'], 1);
  });

  test('status 1 is not ok', () {
    final r = ApiResponse<Object?>.fromJson(
      {'status': 1, 'message': 'bad', 'data': null},
      (d) => d,
    );
    expect(r.isOk, isFalse);
    expect(r.message, 'bad');
  });

  test('null data does not call parse', () {
    var called = false;
    final r = ApiResponse<Object?>.fromJson(
      {'status': 0, 'message': '', 'data': null},
      (d) {
        called = true;
        return d;
      },
    );
    expect(r.data, isNull);
    expect(called, isFalse);
  });
}
