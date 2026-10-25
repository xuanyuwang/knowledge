export interface AssistanceUsedLeaderboardRow {
  info?: unknown; // Added as property, because we have an accessor 'info' in the table.

  resourceName: string;
  team: string;
  assistanceUsed?: number;
  overallEngagement?: number;

  numOfConvos?: number;
  conversationsWithAssistanceUsedPercent?: number;
  numOfConvosPoweredByAA?: number;
  avgConvosPerDay?: number;
  convosPoweredByAARate?: number;
  activeDays?: number;
  averageHandleTime?: number;

  hintsFollowed?: number;
  behavioralHintSent?: number;
  behavioralHintFollowed?: number;
  behavioralHintFollowRate?: number;
  kbHintSent?: number;
  kbHintFollowed?: number;
  kbHintFollowRate?: number;
  gwfHintSent?: number;
  gwfHintFollowed?: number;
  gwfHintFollowRate?: number;
  checklistHintSent?: number;
  checklistHintFollowed?: number;
  checklistHintFollowRate?: number;

  numOfSummaryUsed?: number;
  numOfSummarySent?: number;
  summaryUsageRate?: number;
  averageConvoWithSummary?: number;

  genAiAnswersUsed?: number;
  genAiAnswersViewed?: number;
  knowledgeMomentDetectedCount?: number;
  genAiAnswersUsageRate?: number;
  genAiAnswersUsedPerFrequency?: number;

  searchesPerFrequency?: number;
  gwPerFrequency?: number;
  suggestionsPerFrequency?: number;
  smartComposePerFrequency?: number;
  notesPerFrequency?: number;
  summaryPerFrequency?: number;

  totalHintsFollowed?: number;
  totalHintsSent?: number;
  totalHintsFollowedRate?: number;
}

export interface AgentAssistanceUsedLeaderboardRow extends AssistanceUsedLeaderboardRow {
  icon?: unknown; // Added as property, because we have an accessor 'icon' in the table.

  username: string;
  name: string;
}

export interface TeamAssistanceUsedLeaderboardRow extends AssistanceUsedLeaderboardRow {
  teamResourceName: string;
}
