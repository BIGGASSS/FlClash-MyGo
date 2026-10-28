// Export generated upstream Intl messages for every cardinal-plural catalog entry.
import 'dart:convert';
import 'dart:io';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:fl_clash/l10n/l10n.dart';
void main() {
  test('export original plural messages', () async {
    final output=<Map<String,Object>>[];
    for (final locale in ['en','es','fr','ru','ja','ko','zh_CN']) {
      final parts=locale.split('_');
      final l=await AppLocalizations.load(Locale(parts.first,parts.length>1?parts.last:null));
      for (final n in <num>[-1,0,1,1.5,2,3,5,11,21,101,1000]) {
        output.addAll([
        {'locale':locale,'key':"accessControlSelectedCount",'values':{"count":n},'expected':l.accessControlSelectedCount(n)},
        {'locale':locale,'key':"yearsAgo",'values':{"count":n},'expected':l.yearsAgo(n)},
        {'locale':locale,'key':"monthsAgo",'values':{"count":n},'expected':l.monthsAgo(n)},
        {'locale':locale,'key':"daysAgo",'values':{"count":n},'expected':l.daysAgo(n)},
        {'locale':locale,'key':"hoursAgo",'values':{"count":n},'expected':l.hoursAgo(n)},
        {'locale':locale,'key':"minutesAgo",'values':{"count":n},'expected':l.minutesAgo(n)},
        {'locale':locale,'key':"yearsAgoShort",'values':{"count":n},'expected':l.yearsAgoShort(n)},
        {'locale':locale,'key':"monthsAgoShort",'values':{"count":n},'expected':l.monthsAgoShort(n)},
        {'locale':locale,'key':"daysAgoShort",'values':{"count":n},'expected':l.daysAgoShort(n)},
        {'locale':locale,'key':"hoursAgoShort",'values':{"count":n},'expected':l.hoursAgoShort(n)},
        {'locale':locale,'key':"minutesAgoShort",'values':{"count":n},'expected':l.minutesAgoShort(n)},
        {'locale':locale,'key':"secondsCount",'values':{"count":n},'expected':l.secondsCount(n)},
        {'locale':locale,'key':"proxiesCount",'values':{"count":n,"label":"Example"},'expected':l.proxiesCount(n,"Example")},
        {'locale':locale,'key':"rulesCount",'values':{"count":n,"label":"Example"},'expected':l.rulesCount(n,"Example")},
        {'locale':locale,'key':"proxyGroupsCount",'values':{"count":n,"label":"Example"},'expected':l.proxyGroupsCount(n,"Example")},
        {'locale':locale,'key':"settingsCount",'values':{"count":n},'expected':l.settingsCount(n)},
        {'locale':locale,'key':"itemsCount",'values':{"count":n},'expected':l.itemsCount(n)},
        {'locale':locale,'key':"serviceAbnormalCount",'values':{"count":n},'expected':l.serviceAbnormalCount(n)},
        {'locale':locale,'key':"groupCount",'values':{"count":n},'expected':l.groupCount(n)},
        {'locale':locale,'key':"profileExpiresIn",'values':{"date":"2030-01-02","days":n},'expected':l.profileExpiresIn("2030-01-02",n)},
        {'locale':locale,'key':"profileDaysLeft",'values':{"days":n},'expected':l.profileDaysLeft(n)},
        {'locale':locale,'key':"customImportDroppedSubRules",'values':{"count":n,"label":"Example"},'expected':l.customImportDroppedSubRules(n,"Example")},
        ]);
      }
    }
    await File(Platform.environment['FLCLASH_MESSAGES_PATH']!).writeAsString(jsonEncode(output));
  });
}
