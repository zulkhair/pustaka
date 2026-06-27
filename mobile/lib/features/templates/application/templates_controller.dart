import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../data/models/template.dart';
import '../data/template_repository.dart';

class TemplatesController extends AsyncNotifier<List<Template>> {
  @override
  Future<List<Template>> build() =>
      ref.read(templateRepositoryProvider).fetch();
}

final templatesControllerProvider =
    AsyncNotifierProvider<TemplatesController, List<Template>>(
        TemplatesController.new);
