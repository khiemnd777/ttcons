import { useQuery } from '@tanstack/react-query';
import { get } from '../../lib/api/client';
import type { Project } from '../../lib/types/domain';

export function ProjectSelector({ value, onChange, placeholder = 'Chọn công trình', label = 'Công trình' }: { value?: string; onChange: (id: string) => void; placeholder?: string; label?: string }) {
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Project[]>('/projects') });
  return (
    <select
      aria-label={label}
      className="rounded-md border border-line bg-white px-3 py-2 text-sm focus-ring"
      value={value ?? ''}
      onChange={(event) => onChange(event.target.value)}
    >
      <option value="">{placeholder}</option>
      {projects.data?.map((project) => (
        <option key={project.id} value={project.id}>
          {project.name}
        </option>
      ))}
    </select>
  );
}
