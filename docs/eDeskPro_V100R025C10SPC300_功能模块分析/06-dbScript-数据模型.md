# 06 · `dbScript/` 数据模型

> 57 个 SQL（global/13、network/22、unicnetwork/12、unicollect/、log/），定义 eDesk Pro 全量数据库 schema。
> 注意：`network/1.2.2.sql` 是采集子系统的全量镜像，与 `unicnetwork/`、`unicollect/` 中表**高度重复**；移植时以 **unicnetwork/unicollect 为权威源**，避免重复建表。

## 1. 全部表清单（按子目录）

**global/**：`TBL_PROXY_DEVICE`、`TBL_PROXY`、`TBL_COMMON`、`tbl_networkinfo`、`tbl_Authinfo`、`TBL_USER_INFO`、`tbl_permission_display`、`tbl_audit_log`、`tbl_authentication_info`、`tbl_xftp_device`、`tbl_ne_connect_lock`、`tbl_password_storage_policy`、`version`。

**network/**（巡检核心）：`tbl_ne_common`、`tbl_protocol_ssh/telnet/serial/mml/snmp/ipmi/https/vmm`、`tbl_device_info_bmc/os/ies/switch/pod_manager`、`tbl_sn`、`tbl_ne_optional_param`、`tbl_ne_analysis`、`tbl_ne_group`、`tbl_task`、`tbl_task_schedule_strategy_info`、`tbl_cycle_task_scheduler_strategy_info/map`、`tbl_task_process`、`tbl_task_device`、`tbl_task_device_cycle`、`TBL_ASSET_DEVICE_INFO_OFFLINE`、`TBL_NE_COMMON_TEMP`、`tbl_login_info`、`tbl_project`。

**unicnetwork/**（采集子系统）：`tbl_collect_task_entity`、`tbl_collect_task_item_entity`、`tbl_collect_item_param`、`tbl_collect_item_param2task/2template_relation`、`tbl_collect_task2datasource_relation`、`tbl_global_config_param`、`tbl_system_task`、`tbl_collect_template_entity/param`、`tbl_collect_item_group_entity`、`tbl_collect_item2group_relation`、`tbl_collect_template_item_entity`、`tbl_collect_item_entity`、`tbl_collect_script_entity`、`tbl_ftp_info_entity`、`tbl_ftp_task_entity`、`tbl_ne_entity`、`tbl_connection_entity`、`tbl_resource_entity`、`tbl_ne_protocol_*`、`tbl_collect_task_ne_once/period_statistic_entity`、`tbl_collect_result_entity`、`tbl_configurable_parameter`、`tbl_task_schedule_strategy_info_app`、`tbl_plugin_entity`、`tbl_agent_entity`。

**unicollect/**（采集管理）：`tbl_task_configuration_content_entity`、`tbl_whitelist`、`legacy_ne_table`、`collect_region_group`、`tbl_ne_certificate`、`collect_agent_param`、`tbl_snmpset_record`、`tbl_process_lock_entity`、`tbl_agent_entity`、`tbl_collect_instance*_entity`、`tbl_collect_request*_entity/group*_relation`、`tbl_ne_entity`、`tbl_connection_entity`、`tbl_ne_app_relation`、`tbl_ne_subscription_entity`、`tbl_ne_protocol_*_param`、`tbl_ne_res_entity`、`tbl_ops_packet_statistics_{second..monthly}`、`tbl_plugin_entity`、`tbl_pipeline_entity`、`tbl_plugin_custom_config_entity`、`tbl_security_config_param`、`tbl_connector_entity`、`tbl_ne_connector_relation`、`tbl_ocd_entity/_pipeline/_plugin_pipeline/...`、`tbl_collect_plugin_rule`、`tbl_collect_label`、`TBL_NE_DESTINATION_*`、`TBL_FRAME_*`、`TBL_AGENT_SYS_TASK`、`TBL_COLLECT_*`、`TBL_ASYNC_TASK`、`TBL_NE_*_RECONCILIATION_TASK`。

**log/**：`tbl_audit_log`、`version`。

## 2. 核心表（列 / PK / 说明）

| 表 (源) | 关键列 | 说明 |
|---|---|---|
| `tbl_ne_common` (network) | id PK, ip, name, vendor, type, version, patch_version, base_protocol_*, esn, connect_id | 巡检网元主表 |
| `tbl_ne_entity` (unicnetwork) | id PK, uuid UK, ip, vendor, type, version, hw_version, os_version, gate_way_uuid, dcn_name, native_id | 采集网元主表 |
| `tbl_task` (network) | id PK, taskname, scene, template, ne_type, network_cloud_param, extinfo | 巡检任务 |
| `tbl_task_process` (network) | id PK, status, process, totalcount, successcount, failedcount, resultpath, error_msg | 任务进度/状态 |
| `tbl_task_device` (network) | id PK, **taskid FK→tbl_task.id**, taskdeviceid, status, collectresult, template_id | 任务↔设备执行行 |
| `tbl_ne_analysis` (network) | id PK, item_id, inspect_device_id, result, level, resion, solution, item_description, **item_packets CLOB(原始回显)**, inspect_type | 巡检结果/明细 |
| `tbl_collect_task_entity` (unicnetwork) | id PK, uuid UK, name, scene, template_uuid, status, start/end_time, user_id | 采集任务 |
| `tbl_collect_task_item_entity` (unicnetwork) | id PK, **collect_task_uuid FK**, ne_uuid, collect_item_uuid, status, rounds | 采集项执行明细 |
| `tbl_collect_item_entity` (unicnetwork) | id PK, uuid UK, name, data_source_type, priority, resource_type | 采集项定义 |
| `tbl_collect_template_entity` (unicnetwork) | id PK, uuid UK, name, scene, type, status, parent_template_uuid, mandatory, dcn_check | 采集模板 |
| `tbl_collect_template_item_entity` (unicnetwork) | id PK, **template_uuid FK**, collect_item_uuid, item_type, name, selection_flag | 模板↔采集项 |
| `tbl_collect_task_ne_relation` (unicnetwork) | id PK, **collect_task_uuid, ne_uuid**, status, request_success/fail/miss, period_type | 采集任务↔网元 |
| `tbl_collect_result_entity` (unicnetwork) | id PK, uuid, **collect_task_id FK**, file_name | 采集结果文件(原始回显包) |
| `tbl_agent_entity` (unicnetwork) | id PK, uuid UK, group_id, type, ip, port, status, name | 采集器/Agent |
| `tbl_protocol_ssh/telnet/serial/mml` (network) | ne_common_id PK **FK→tbl_ne_common ON DELETE CASCADE**, username, password, port | 网元协议凭据 |
| `tbl_protocol_snmp` (network) | id PK, port, version, community_*, username | SNMP 凭据 |
| `tbl_device_info_bmc/os/ies/switch/pod_manager` (network) | ne_common_id PK **FK**, serial_number, product_name, version, esn, software, blades_info | 硬件详情(类 CEAS/elabel) |
| `tbl_ne_whitelist` (unicnetwork) | id PK, uuid UK, esn, type, version, license_type, mac | 网元白名单(含 esn/硬件标识) |
| `tbl_ne_certificate` (unicollect) | id PK, ne_type, ne_version, certificate_ca_id | 设备版本→证书映射 |
| `tbl_ocd_ne_model_entity` (unicollect) | id PK, ne_type, ne_version, ne_model_uuid | 设备版本→模型/管线映射 |
| `tbl_plugin_entity` (unicnetwork) | id PK, plugin_id UK, type, vendor, content BLOB, pkg_name, status | 插件/驱动(解析规则载体) |
| `tbl_pipeline_entity` (unicollect) | id PK, pipeline_id UK, content CLOB, vendor, connect_type | 管线(input/parser/output 链) |
| `TBL_COLLECT_PLUGIN_RULE` (unicollect) | id PK, pipeline_id, plugin_type, rule_type, rule_id, **rule_content CLOB**, status | 采集解析规则(阈值/规则) |
| `TBL_PROXY_DEVICE` / `TBL_PROXY` (global) | TBL_PROXY_DEVICE: id PK, PROXY_ID, IP, PORT, USERNAME, **CMD(命令)**, **EXTEND_CMD(高级/风险命令)**, VPN | 代理网关 + 风险命令 |
| `TBL_USER_INFO` / `tbl_login_info` / `tbl_authentication_info` (global) | uuid/PK, password, permissionLevel, sign; username,password; esn, level | 用户/账号/鉴权 |
| `tbl_audit_log` (global/log) | id PK, operation, username, source, level, type, result, ip, detail | 审计日志 |
| `tbl_dcn_info` / `tbl_dcn_task_info` (unicnetwork/2.3.3) | dcn name/ne_size/risky_link_num; uuid/status/detection/restoration/collect_task_uuid | DCN 探测/还原(风险链路) |
| `tbl_project` (network) | id PK, project_name, ne_ids, app_name, scene_zh/en | 项目↔网元分组 |

## 3. 外键 / 关系线索

- 任务→设备：`tbl_task_device.taskid → tbl_task.id`；`tbl_task_device_cycle.taskid → tbl_task.id`。
- 结果→任务/设备：`tbl_ne_analysis.inspect_device_id → 设备`；`tbl_collect_result_entity.collect_task_id → tbl_collect_task_entity.uuid`；`tbl_collect_task_item_entity.collect_task_uuid → task`；`tbl_collect_task_ne_relation(collect_task_uuid, ne_uuid)`。
- 设备→协议/硬件：`tbl_protocol_*/tbl_device_info_*.ne_common_id → tbl_ne_common(id) ON DELETE CASCADE`；采集侧 `tbl_ne_protocol_*_param.ne_uuid → tbl_ne_entity(uuid) ON DELETE CASCADE`；`tbl_connection_entity.ne_uuid → tbl_ne_entity(uuid)`。
- 模板→采集项：`tbl_collect_template_item_entity.template_uuid → tbl_collect_template_entity`；`tbl_collect_item2group_relation` 关联。
- Agent→Request：`tbl_collect_request_group2agent_relation.agent_uuid → tbl_agent_entity`；`tbl_ne_agent_relation(ne_uuid, agent_id)`。

## 4. 特殊用途表定位（直接对应移植需求）

| 需求 | 表 |
|---|---|
| 命令原始回显 | `tbl_ne_analysis.item_packets`(CLOB)、`tbl_collect_result_entity.file_name`(回显包)、`tbl_task_process.resultpath` |
| 解析规则 | `TBL_COLLECT_PLUGIN_RULE`(RULE_CONTENT CLOB)、`tbl_plugin_entity`(BLOB)、`tbl_pipeline_entity`(CLOB)、`tbl_ocd_*` |
| 阈值 | 无独立阈值表，内嵌于 `TBL_COLLECT_PLUGIN_RULE.RULE_CONTENT` / 模板参数 |
| 风险命令 | `TBL_PROXY_DEVICE.CMD` / `EXTEND_CMD`、`tbl_snmpset_record.set_cmd`、`tbl_dcn_info.risky_link_num` |
| 设备版本映射 | `tbl_ne_certificate`、`tbl_ocd_ne_model_entity`、`tbl_ne_protocol_config_type` |
| CEAS/elabel 硬件 | 无专用 CEAS 表；硬件信息分散在 `tbl_device_info_bmc/os/ies/switch/pod_manager` 与 `tbl_ne_whitelist`(esn) |

## 5. 移植要点（NetWeaverGo）

1. **设备主表**：合并 `tbl_ne_common` + `tbl_ne_entity` 为单一「网元」模型（含 vendor/type/version/esn/协议凭据）。
2. **任务模型**：`tbl_task` + `tbl_task_device` + `tbl_task_process` 对应「巡检任务/执行行/进度」；采集侧 `tbl_collect_task_*` 对应「采集任务」。
3. **采集项/模板**：`tbl_collect_item_entity` + `tbl_collect_template_entity` + `tbl_collect_template_item_entity` 是「检查项/模板」核心，直接对应 NetWeaverGo 的模板引擎。
4. **结果模型**：`tbl_ne_analysis`(巡检结果 + 原始回显 CLOB) + `tbl_collect_result_entity`(回显包) 对应「检查结果/原始报文」存储。
5. **规则/插件**：`TBL_COLLECT_PLUGIN_RULE` + `tbl_pipeline_entity` 是「解析规则/管线」载体，建议 NetWeaverGo 以 JSON/结构化表存储规则。
6. **优先去重**：以 `unicnetwork/unicollect` 为权威源建表，忽略 `network/1.2.2.sql` 镜像。
