// Run in the pinned Flutter checkout; see README.md.
import 'dart:convert';
import 'dart:io';

import 'package:fl_clash/common/common.dart';
import 'package:fl_clash/models/models.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:yaml/yaml.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  test('export original runtime assembly', () async {
    final cases = <Map<String, dynamic>>[
      {'name': 'empty'},
      {
        'name': 'disabled-dns',
        'raw': {
          'dns': {
            'enable': false,
            'nameserver': ['9.9.9.9'],
            'listen': ':1053',
            'future-key': true,
          },
        },
      },
      {
        'name': 'enabled-dns',
        'raw': {
          'dns': {
            'enable': true,
            'nameserver': ['9.9.9.9'],
            'fallback-filter': {
              'geoip': false,
              'domain': ['keep.me'],
            },
          },
        },
      },
      {
        'name': 'optional-inherited',
        'raw': {
          'unified-delay': false,
          'tcp-concurrent': false,
          'ipv6': true,
          'geodata-loader': 'standard',
          'test-url': 'https://profile.test/204',
        },
      },
      {
        'name': 'optional-selected',
        'raw': {
          'unified-delay': false,
          'tcp-concurrent': false,
          'ipv6': true,
          'geodata-loader': 'standard',
        },
        'patch': {
          'tcp-concurrent': true,
          'general-override-keys': ['tcp-concurrent', 'test-url'],
          'test-url': 'https://selected.test/204',
        },
      },
      {
        'name': 'app-owned-listeners',
        'raw': {
          'mixed-port': 1234,
          'port': 1235,
          'socks-port': 1236,
          'external-controller': '0.0.0.0:9999',
          'secret': 'profile-secret',
          'external-controller-tls': ':9998',
          'external-controller-unix': '/tmp/profile.sock',
          'external-controller-pipe': 'profile',
          'external-ui': 'ui',
          'external-ui-url': 'https://profile.test',
          'authentication': ['profile:password'],
          'skip-auth-prefixes': ['127.0.0.0/8'],
          'profile': {'store-selected': true, 'store-fake-ip': true},
        },
      },
      {
        'name': 'controller-without-secret',
        'patch': {'external-controller': '127.0.0.1:9090'},
      },
      {
        'name': 'controller-with-secret',
        'patch': {
          'external-controller': '127.0.0.1:9090',
          'secret': 'app-secret',
          'mixed-port': 8789,
          'port': 8790,
          'socks-port': 8791,
        },
        'authentication': ['alice:example'],
      },
      {
        'name': 'interface-clear',
        'raw': {'interface-name': 'profile0'},
        'patch': {
          'interface-name': 'remembered0',
          'interface-name-mode': 'clear',
        },
      },
      {
        'name': 'interface-follow',
        'raw': {'interface-name': 'profile0'},
        'patch': {
          'interface-name': 'remembered0',
          'interface-name-mode': 'follow',
        },
      },
      {
        'name': 'interface-custom',
        'raw': {'interface-name': 'profile0'},
        'patch': {'interface-name': 'custom0', 'interface-name-mode': 'custom'},
      },
      {
        'name': 'tun-app-core',
        'raw': {
          'tun': {
            'enable': true,
            'device': 'profile',
            'stack': 'gvisor',
            'auto-route': false,
            'auto-detect-interface': false,
            'dns-hijack': ['tcp://1.1.1.1:53'],
            'mtu': 1800,
            'strict-route': true,
            'future-key': 'preserved',
          },
        },
      },
      {
        'name': 'tun-selected',
        'raw': {
          'tun': {'mtu': 1800, 'strict-route': true},
        },
        'patch': {
          'tun-override': {'mtu': 1500, 'strict-route': false},
          'tun-override-keys': ['mtu', 'strict-route'],
        },
      },
      {
        'name': 'dns-selected',
        'raw': {
          'dns': {
            'enable': true,
            'nameserver': ['9.9.9.9'],
            'listen': ':1053',
            'fallback-filter': {
              'geoip': false,
              'domain': ['keep.me'],
            },
          },
        },
        'patch': {
          'dns': {
            'ipv6': true,
            'nameserver': ['1.1.1.1'],
          },
          'dns-override-keys': ['ipv6', 'nameserver'],
        },
      },
      {
        'name': 'sniffer-normalization',
        'raw': {
          'sniffer': {
            'enable': true,
            'sniff': {
              'HTTP': {
                'ports': [80, '8080-8090'],
                'future-key': true,
              },
            },
          },
        },
      },
      {
        'name': 'unknown-fields',
        'raw': {
          'future-option': {
            'preserve': [true, 123, 'abc'],
          },
          'ntp': {'enable': true, 'server': 'clock.test'},
          'hosts': {'profile.test': '192.0.2.1'},
        },
      },
      {
        'name': 'dns-empty-nameserver',
        'raw': {
          'dns': {
            'enable': true,
            'nameserver': ['9.9.9.9'],
          },
        },
        'patch': {
          'dns': {'nameserver': <String>[]},
          'dns-override-keys': ['nameserver'],
        },
      },
      {
        'name': 'dns-empty-fallback',
        'raw': {
          'dns': {
            'enable': true,
            'nameserver': ['9.9.9.9'],
            'fallback': ['8.8.8.8'],
          },
        },
        'patch': {
          'dns': {'fallback': <String>[]},
          'dns-override-keys': ['fallback'],
        },
      },
      {
        'name': 'dns-empty-listen',
        'raw': {
          'dns': {
            'enable': true,
            'nameserver': ['9.9.9.9'],
            'listen': ':1053',
          },
        },
        'patch': {
          'dns': {'listen': ''},
          'dns-override-keys': ['listen'],
        },
      },
      {
        'name': 'dns-policy-replacement',
        'raw': {
          'dns': {
            'enable': true,
            'nameserver': ['9.9.9.9'],
            'nameserver-policy': {
              'old.test': ['8.8.8.8'],
            },
          },
        },
        'patch': {
          'dns': {
            'nameserver-policy': {'new.test': '1.1.1.1, 8.8.8.8'},
          },
          'dns-override-keys': ['nameserver-policy'],
        },
      },
      {
        'name': 'host-lists',
        'raw': {
          'hosts': {'profile.test': '192.0.2.1'},
        },
        'patch': {
          'hosts': {
            'new.test': '192.0.2.2, 192.0.2.3;192.0.2.4',
            'single.test': '192.0.2.5',
          },
        },
      },
      {
        'name': 'bypass-private-off',
        'raw': {
          'tun': {
            'route-exclude-address': ['198.51.100.0/24'],
          },
        },
        'bypassPrivateRoute': false,
      },
      {
        'name': 'append-system-dns',
        'raw': {
          'dns': {
            'enable': true,
            'nameserver': ['9.9.9.9'],
          },
        },
        'appendSystemDns': true,
      },
      {
        'name': 'system-dns-no-duplicate',
        'raw': {
          'dns': {
            'enable': true,
            'nameserver': ['system://', '9.9.9.9'],
          },
        },
        'appendSystemDns': true,
      },
      {
        'name': 'iptables-interface-list',
        'raw': {
          'iptables': {
            'enable': true,
            'inbound-interface': 'eth0 eth1',
            'future': true,
          },
        },
      },
      {
        'name': 'sniffer-protocol-replaces-entry',
        'raw': {'sniffer': {'enable': true, 'sniff': {
          'HTTP': {'ports': [80], 'override-destination': true, 'future-key': true},
          'TLS': {'ports': [443]},
        }}},
        'overrides': {'sniffer': {'sniff': {'HTTP': {'ports': [8080]}}}, 'sniffer-override-keys': ['sniff.HTTP']},
      },
      {
        'name': 'sniffer-case-insensitive',
        'raw': {'sniffer': {'sniff': {'http': {'ports': [80]}, 'HtTp': {'ports': [81]}, 'TLS': {'ports': [443]}}}},
        'overrides': {'sniffer': {'sniff': {'HTTP': {'ports': [8080], 'override-destination': false}}}, 'sniffer-override-keys': ['sniff.HTTP']},
      },
      {
        'name': 'sniffer-legacy-protocols',
        'raw': {'sniffer': {'sniffing': ['http', 'tls'], 'port-whitelist': [80, '443', '8000-9000']}},
        'overrides': {'sniffer': {'sniff': {'TLS': {'ports': [8443]}}}, 'sniffer-override-keys': ['sniff.TLS']},
      },
      {
        'name': 'sniffer-empty-map-uses-legacy',
        'raw': {'sniffer': {'sniff': {}, 'sniffing': ['HTTP', 'QUIC']}},
        'overrides': {'sniffer': {'sniff': {'TLS': {'ports': [443]}}}, 'sniffer-override-keys': ['sniff.TLS']},
      },
      {
        'name': 'sniffer-map-precedes-legacy',
        'raw': {'sniffer': {'sniff': {'HTTP': {'ports': [80]}}, 'sniffing': ['QUIC'], 'port-whitelist': [443]}},
        'overrides': {'sniffer': {'sniff': {'TLS': {'ports': [8443]}}}, 'sniffer-override-keys': ['sniff.TLS']},
      },
      {
        'name': 'sniffer-list-replacement',
        'raw': {'sniffer': {'skip-domain': ['old.test'], 'force-domain': ['keep.test']}},
        'overrides': {'sniffer': {'skip-domain': ['new.test']}, 'sniffer-override-keys': ['skip-domain']},
      },
      {
        'name': 'sniffer-enable-keeps-legacy-shape',
        'raw': {'sniffer': {'enable': true, 'sniffing': ['HTTP'], 'port-whitelist': [80]}},
        'overrides': {'sniffer': {'enable': false}, 'sniffer-override-keys': ['enable']},
      },
      {
        'name': 'rule-target-follows-last-match',
        'raw': {'rules': ['MATCH,First', 'DOMAIN,base.test,DIRECT', 'match,Latest', 'MATCH,']},
        'addedRules': ['DOMAIN,added.test,MATCH', 'IP-CIDR,192.0.2.0/24,match,src,no-resolve'],
      },
      {
        'name': 'rule-explicit-match-target',
        'raw': {'rules': ['MATCH,Original']},
        'matchTarget': '  Chosen  ',
        'addedRules': ['DOMAIN,added.test,MATCH', 'MATCH,MATCH'],
      },
      {
        'name': 'rule-missing-match-keeps-placeholder',
        'raw': {'rules': ['DOMAIN,base.test,DIRECT']},
        'addedRules': ['DOMAIN,added.test,MATCH'],
      },
      {
        'name': 'rule-comma-payload-and-subrule',
        'raw': {'rules': ['MATCH,Chosen']},
        'addedRules': ['AND,((DOMAIN,a.test),(DOMAIN,b.test)),MATCH', r'DOMAIN-REGEX,^x{1,3}\.test$,MATCH', 'SUB-RULE,(NETWORK,TCP),MATCH'],
      },
    ];
    final output = <Object>[];
    for (final item in cases) {
      final raw = <String, dynamic>{
        'proxies': <Object>[],
        'proxy-groups': <Object>[],
        'rules': ['MATCH,DIRECT'],
        ...?item['raw'] as Map<String, dynamic>?,
      };
      final patch = PatchClashConfig.fromJson({
        'general-override-keys': <String>[],
        ...?item['patch'] as Map<String, dynamic>?,
      });
      final auth =
          (item['authentication'] as List?)?.cast<String>() ?? <String>[];
      final result = await makeRealProfileTask(
        MakeRealProfileState(
          profilesPath: Directory.systemTemp.path,
          profileId: 1,
          rawConfig: jsonDecode(jsonEncode(raw)) as Map<String, dynamic>,
          realPatchConfig: patch,
          overrides: ProfileOverrides.fromJson(
            (item['overrides'] as Map<String, dynamic>?) ?? {},
          ).copyWith(
            dns: patch.dns,
            dnsOverrideKeys: patch.dnsOverrideKeys,
            tun: patch.tunOverride,
            tunOverrideKeys: patch.tunOverrideKeys,
          ),
          keepEmptyDnsOverrides: true,
          authentication: auth,
          bypassPrivateRoute: item['bypassPrivateRoute'] as bool? ?? true,
          appendSystemDns: item['appendSystemDns'] as bool? ?? false,
          proxyGroups: const [],
          rules: const [],
          addedRules: [for (final rule in (item['addedRules'] as List?) ?? []) Rule.parse(rule as String)],
          matchTarget: item['matchTarget'] as String?,
          defaultUA: 'FlClash/MyGo',
        ),
      );
      output.add({
        'name': item['name'],
        'raw': raw,
        'patch': patch.toJson(),
        'authentication': auth,
        'bypassPrivateRoute': item['bypassPrivateRoute'] as bool? ?? true,
        'appendSystemDns': item['appendSystemDns'] as bool? ?? false,
        'overrides': item['overrides'],
        'addedRules': item['addedRules'],
        'matchTarget': item['matchTarget'],
        'expected': loadYaml(result.yaml),
      });
    }
    final target = File(Platform.environment['FLCLASH_RUNTIME_PATH']!);
    await target.parent.create(recursive: true);
    await target.writeAsString(jsonEncode(output));
  });
}
