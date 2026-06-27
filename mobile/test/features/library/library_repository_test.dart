import 'package:flutter_test/flutter_test.dart';
import 'package:pustaka/features/library/data/library_repository.dart';

import '../../support/fake_api.dart';

void main() {
  test('fetch marks owned isOwner=true and shared isOwner=false', () async {
    final client = apiClientReturningData({
      'owned': [
        {'id': 'o1', 'title': 'Owned', 'mode': 'photo', 'status': 'done'},
      ],
      'shared': [
        {'id': 's1', 'title': 'Shared', 'mode': 'text', 'status': 'done'},
      ],
    });
    final repo = LibraryRepository(client);
    final docs = await repo.fetch();
    expect(docs.owned.single.id, 'o1');
    expect(docs.owned.single.isOwner, isTrue);
    expect(docs.shared.single.id, 's1');
    expect(docs.shared.single.isOwner, isFalse);
  });
}
