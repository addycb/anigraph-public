export interface GraphNode {
  id: string | number;
  label: string;
  type: "anime" | "staff" | "category";
  image?: string;
  group: string;
  category?: string;
  format?: string | null;
  averageScore?: number | null;
  seasonYear?: number | null;
  season?: string | null;
  fx?: number | null;
  fy?: number | null;
  x?: number;
  y?: number;
  // Category node specific fields
  staffCount?: number;
  staffList?: Array<{
    id: string;
    name: string;
    image?: string;
    role?: string;
  }>;
}

export interface GraphLink {
  source: string | number | GraphNode;
  target: string | number | GraphNode;
  role?: string | string[];
  type: string;
  category?: string;
  // Category node link specific fields
  staffCount?: number;
  staffNames?: string[];
  categoryBreakdown?: Record<string, number>;
  staffDetails?: Array<{
    staffId: string;
    staffName: string;
    image?: string;
    mainRole: string;
    otherRole: string;
    category: string;
  }>;
  parallelOffset?: number;
}

export interface GraphData {
  nodes: GraphNode[];
  links: GraphLink[];
  center: string | number;
  minConnectionsFloor?: number;
}
