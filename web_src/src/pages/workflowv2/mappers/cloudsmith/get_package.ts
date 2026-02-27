import {
  ComponentBaseContext,
  ComponentBaseMapper,
  ExecutionDetailsContext,
  NodeInfo,
  OutputPayload,
  SubtitleContext,
} from "../types";
import { ComponentBaseProps } from "@/ui/componentBase";
import { getBackgroundColorClass, getColorClass } from "@/utils/colors";
import { getStateMap } from "..";
import { formatTimeAgo } from "@/utils/date";
import { MetadataItem } from "@/ui/metadataList";
import { PackageInfo } from "./types";
import { formatBytes, stringOrDash } from "../utils";
import cloudsmithIcon from "@/assets/icons/integrations/cloudsmith.svg";

interface GetPackageConfiguration {
  repository?: string;
  identifier?: string;
}

export const getPackageMapper: ComponentBaseMapper = {
  props(context: ComponentBaseContext): ComponentBaseProps {
    const lastExecution = context.lastExecutions.length > 0 ? context.lastExecutions[0] : null;
    const componentName = context.componentDefinition.name || "unknown";

    return {
      title: context.node.name || context.componentDefinition.label || "Unnamed component",
      iconSrc: cloudsmithIcon,
      iconColor: getColorClass(context.componentDefinition.color),
      collapsedBackground: getBackgroundColorClass(context.componentDefinition.color),
      collapsed: context.node.isCollapsed,
      eventSections: lastExecution ? [] : undefined,
      includeEmptyState: !lastExecution,
      metadata: getPackageMetadataList(context.node),
      eventStateMap: getStateMap(componentName),
    };
  },

  getExecutionDetails(context: ExecutionDetailsContext): Record<string, string> {
    const outputs = context.execution.outputs as { default?: OutputPayload[] } | undefined;
    const result = outputs?.default?.[0]?.data as PackageInfo | undefined;

    if (!result) {
      return {};
    }

    return {
      Name: stringOrDash(result.name),
      Version: stringOrDash(result.version),
      Format: stringOrDash(result.format),
      Size: formatBytes(result.size),
      Status: stringOrDash(result.status_str),
      "CDN URL": stringOrDash(result.cdn_url),
      "SHA-256": stringOrDash(result.checksum_sha256),
    };
  },

  subtitle(context: SubtitleContext): string {
    if (!context.execution.createdAt) {
      return "";
    }
    return formatTimeAgo(new Date(context.execution.createdAt));
  },
};

function getPackageMetadataList(node: NodeInfo): MetadataItem[] {
  const metadata: MetadataItem[] = [];
  const configuration = node.configuration as GetPackageConfiguration | undefined;

  if (configuration?.repository) {
    metadata.push({ icon: "package", label: configuration.repository });
  }

  if (configuration?.identifier) {
    metadata.push({ icon: "tag", label: configuration.identifier });
  }

  return metadata;
}
