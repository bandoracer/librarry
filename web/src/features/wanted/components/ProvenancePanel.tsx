import React, { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { CheckCircle2 } from "lucide-react";
import { applyWantedMetadataCorrection, applyWantedMetadataCorrections } from "../../../lib/api";
import type {
  MetadataFieldCandidate,
  MetadataFieldEvidence,
  MetadataProvenance,
  ProviderMetadataRecord,
  WantedItem
} from "../../../lib/api";
import { keys, useWantedMetadata } from "../../../lib/queries";
import { useToast } from "../../../components/toast";
import { Badge, Button, Card, EmptyState, InlineNotice, LoadingRow } from "../../../components/ui";
import {
  appErrorMessage,
  errorMessage,
  metadataConfidenceLabel,
  metadataFieldApplicableCandidates,
  metadataFieldCanApply,
  metadataFieldCanConfirmCanonical,
  metadataFieldCandidateSummary,
  metadataFieldCanonicalActionID,
  metadataFieldSourceLabel,
  metadataFieldStatusLabel,
  metadataProvenanceSummary,
  metadataRecordActionID,
  metadataRecordCorrections,
  metadataRecordPrimaryLine,
  metadataRecordSecondaryLine
} from "../lib";
import "../wanted.css";

/**
 * Provider provenance panel for one wanted item: per-field evidence with
 * apply/keep-current actions and the stored provider records. Extracted from
 * the legacy BooksTab detail panel; owns its own fetching and mutations.
 */
export function ProvenancePanel(props: { item: WantedItem }) {
  const { item } = props;
  const toast = useToast();
  const client = useQueryClient();
  const metadataQuery = useWantedMetadata(item.id);

  const [applyingCandidateID, setApplyingCandidateID] = useState("");
  const [applyingRecordID, setApplyingRecordID] = useState("");

  const metadata = metadataQuery.data ?? null;
  const isLoadingMetadata = metadataQuery.isLoading;
  const metadataError = metadataQuery.error
    ? appErrorMessage(errorMessage(metadataQuery.error, "Wanted metadata provenance failed"))
    : "";

  function invalidate(...queryKeys: readonly (readonly unknown[])[]) {
    return Promise.all(queryKeys.map((key) => client.invalidateQueries({ queryKey: key })));
  }

  function storeProvenance(provenance: MetadataProvenance) {
    client.setQueryData(keys.wantedMetadata(item.id), provenance);
  }

  async function applyCandidate(field: MetadataFieldEvidence, candidate: MetadataFieldCandidate) {
    if (!metadataFieldCanApply(field)) return;
    const actionID = `${field.fieldName}:${candidate.provider}:${candidate.value}`;
    setApplyingCandidateID(actionID);
    try {
      const provenance = await applyWantedMetadataCorrection(item.id, {
        fieldName: field.fieldName,
        value: candidate.value
      });
      storeProvenance(provenance);
      toast.success(`Applied ${candidate.provider} ${field.label.toLowerCase()}`);
      await invalidate(keys.wanted, keys.wantedMetadataReview, keys.acquisitionQueue);
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Wanted metadata correction failed")));
    } finally {
      setApplyingCandidateID("");
    }
  }

  async function confirmCanonical(field: MetadataFieldEvidence) {
    if (!metadataFieldCanConfirmCanonical(field)) return;
    setApplyingCandidateID(metadataFieldCanonicalActionID(field));
    try {
      const provenance = await applyWantedMetadataCorrection(item.id, {
        fieldName: field.fieldName,
        value: field.canonicalValue || "",
        reason: "metadata review canonical accepted"
      });
      storeProvenance(provenance);
      toast.success(`Kept current ${field.label.toLowerCase()}`);
      await invalidate(keys.wanted, keys.wantedMetadataReview, keys.acquisitionQueue);
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Wanted metadata confirmation failed")));
    } finally {
      setApplyingCandidateID("");
    }
  }

  async function applyRecord(record: ProviderMetadataRecord) {
    const corrections = metadataRecordCorrections(record, metadata);
    if (corrections.length === 0) return;
    setApplyingRecordID(metadataRecordActionID(record));
    try {
      const provenance = await applyWantedMetadataCorrections(item.id, { corrections });
      storeProvenance(provenance);
      toast.success(`Applied ${corrections.length} field${corrections.length === 1 ? "" : "s"} from ${record.provider}`);
      await invalidate(keys.wanted, keys.wantedMetadataReview, keys.acquisitionQueue);
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Wanted metadata corrections failed")));
    } finally {
      setApplyingRecordID("");
    }
  }

  return (
    <Card
      title="Provider provenance"
      subtitle={metadataProvenanceSummary(metadata, isLoadingMetadata)}
      actions={
        metadata?.manualOverrides?.length ? (
          <Badge tone="accent">
            {metadata.manualOverrides.length} override{metadata.manualOverrides.length === 1 ? "" : "s"} protected
          </Badge>
        ) : undefined
      }
    >
      {metadataError ? <InlineNotice tone="danger">{metadataError}</InlineNotice> : null}
      {metadata?.fields.length ? (
        <div className="wanted-provenance-list" aria-label="Metadata field evidence">
          {metadata.fields.map((field) => (
            <article className="wanted-field-row" key={field.fieldName}>
              <div>
                <strong>{field.label}</strong>
                <span>{metadataFieldSourceLabel(field)}</span>
              </div>
              <div>
                <strong>{field.canonicalValue || "No canonical value"}</strong>
                <span>{metadataFieldCandidateSummary(field)}</span>
                {metadataFieldCanConfirmCanonical(field) || metadataFieldApplicableCandidates(field).length ? (
                  <div className="wanted-field-actions" aria-label={`${field.label} provider candidates`}>
                    {metadataFieldCanConfirmCanonical(field) ? (
                      <Button
                        size="sm"
                        busy={applyingCandidateID === metadataFieldCanonicalActionID(field)}
                        onClick={() => void confirmCanonical(field)}
                      >
                        {applyingCandidateID === metadataFieldCanonicalActionID(field) ? "Keeping" : "Keep current"}
                      </Button>
                    ) : null}
                    {metadataFieldApplicableCandidates(field).map((candidate) => {
                      const actionID = `${field.fieldName}:${candidate.provider}:${candidate.value}`;
                      return (
                        <Button
                          size="sm"
                          key={`${candidate.provider}:${candidate.providerKey}:${candidate.value}`}
                          busy={applyingCandidateID === actionID}
                          title={candidate.value}
                          onClick={() => void applyCandidate(field, candidate)}
                        >
                          {applyingCandidateID === actionID ? "Applying" : `Use ${candidate.provider}`}
                        </Button>
                      );
                    })}
                  </div>
                ) : null}
              </div>
              <Badge tone={field.conflict ? "warn" : field.protected ? "accent" : field.reviewResolved ? "success" : "neutral"}>
                {metadataFieldStatusLabel(field)}
              </Badge>
            </article>
          ))}
        </div>
      ) : null}
      {metadata?.records.length ? (
        <div className="wanted-provenance-list" aria-label="Provider records">
          {metadata.records.map((record) => {
            const corrections = metadataRecordCorrections(record, metadata);
            const actionID = metadataRecordActionID(record);
            return (
              <article className="wanted-record-row" key={actionID}>
                <div>
                  <strong>{record.provider}</strong>
                  <span>
                    {record.entityType} · {record.providerKey}
                  </span>
                </div>
                <div>
                  <strong>{metadataRecordPrimaryLine(record)}</strong>
                  <span>{metadataRecordSecondaryLine(record)}</span>
                </div>
                <div className="wanted-record-actions">
                  {corrections.length ? (
                    <Button
                      size="sm"
                      icon={CheckCircle2}
                      disabled={Boolean(applyingRecordID || applyingCandidateID) && applyingRecordID !== actionID}
                      busy={applyingRecordID === actionID}
                      title={`Apply ${corrections.length} metadata field${corrections.length === 1 ? "" : "s"} from ${record.provider}`}
                      onClick={() => void applyRecord(record)}
                    >
                      {applyingRecordID === actionID ? "Applying" : "Use record"}
                    </Button>
                  ) : null}
                  <em>{metadataConfidenceLabel(record.confidence)}</em>
                </div>
              </article>
            );
          })}
        </div>
      ) : isLoadingMetadata ? (
        <LoadingRow label="Loading provenance…" />
      ) : (
        <EmptyState title="No stored provider records">
          Create or refresh this item from metadata search to attach provider records.
        </EmptyState>
      )}
    </Card>
  );
}
