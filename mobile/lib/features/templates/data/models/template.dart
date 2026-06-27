enum TemplateScope { page, document }

enum OutputFormat { markdown, json, csv, text }

TemplateScope templateScopeFromString(Object? s) =>
    s == 'document' ? TemplateScope.document : TemplateScope.page;

OutputFormat outputFormatFromString(Object? s) {
  switch (s) {
    case 'json':
      return OutputFormat.json;
    case 'csv':
      return OutputFormat.csv;
    case 'text':
      return OutputFormat.text;
    default:
      return OutputFormat.markdown;
  }
}

class Template {
  const Template({
    required this.id,
    required this.name,
    required this.scope,
    required this.outputFormat,
    required this.isBuiltin,
    this.docTypeHint,
  });

  final String id;
  final String name;
  final TemplateScope scope;
  final OutputFormat outputFormat;
  final bool isBuiltin;
  final String? docTypeHint;

  factory Template.fromJson(Map<String, dynamic> json) {
    return Template(
      id: json['id'] as String,
      name: json['name'] as String? ?? '',
      scope: templateScopeFromString(json['scope']),
      outputFormat: outputFormatFromString(json['outputFormat']),
      isBuiltin: json['isBuiltin'] as bool? ?? false,
      docTypeHint: json['docTypeHint'] as String?,
    );
  }
}
