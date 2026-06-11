import { LayoutGrid } from 'lucide-react';
import { EnumChip } from '../../../components/EnumChip';

export type NodePreset = {
  type: string;
  name: string;
  description: string;
};

type NodeLibraryPanelProps = {
  presets: NodePreset[];
  canAddNode: boolean;
  addNodePending: boolean;
  onAddNode: (preset: NodePreset) => void;
};

export function NodeLibraryPanel({ presets, canAddNode, addNodePending, onAddNode }: NodeLibraryPanelProps) {
  return (
    <section className="pt-4">
      <div className="mb-3 flex items-center justify-between px-2">
        <h2 className="inline-flex items-center gap-2 text-sm font-semibold text-ink">
          <LayoutGrid className="h-4 w-4 text-slate-400" />
          Thư viện node
        </h2>
      </div>
      <div className="space-y-2">
        {presets.map((preset) => (
          <button
            key={preset.type}
            className="w-full rounded-md border border-line px-3 py-3 text-left hover:border-blue-300 hover:bg-blue-50 disabled:opacity-50"
            disabled={!canAddNode || addNodePending}
            onClick={() => onAddNode(preset)}
          >
            <p className="text-sm font-semibold text-ink">{preset.name}</p>
            <p className="mt-1 line-clamp-2 text-xs leading-5 text-slate-500">{preset.description}</p>
            <div className="mt-2 flex flex-wrap gap-1.5">
              <EnumChip group="node_type" value={preset.type} />
            </div>
          </button>
        ))}
      </div>
    </section>
  );
}
