import { useState } from 'react';
import "./AssignmentSubmissionSetting.css";

const A = '/assets';

const navItems = [
  ['Dashboard', '57bc3.png'],
  ['My Courses', '45c82.png'],
  ['Assignments', '2c8cd.png'],
  ['Results and Grades', '1cad0.png'],
  ['Class Analysis', '4ebe4.png'],
  ['Academic Calender', '2e776.png'],
  ['Notice Board', '4c71a.png'],
] as const;

const dates = ['Allow Submission from', 'Due Date', 'Cut-off date'];

function Check({
  checked,
  onChange,
  label,
}: {
  checked: boolean;
  onChange: () => void;
  label: string;
}) {
  return (
    <button
      type="button"
      className="check-button"
      role="checkbox"
      aria-checked={checked}
      aria-label={label}
      onClick={onChange}
    >
      <img src={`${A}/${checked ? 'dcd8f.png' : '110ec.png'}`} alt="" />
    </button>
  );
}

function DateRow({
  label,
  enabled,
  onToggle,
}: {
  label: string;
  enabled: boolean;
  onToggle: () => void;
}) {
  return (
    <div className={`date-row ${enabled ? '' : 'is-disabled'}`}>
      <span className="date-label">{label} :</span>
      <span className="enable-control">
        <Check checked={enabled} onChange={onToggle} label={`Enable ${label}`} />
        <span>Enable</span>
      </span>
      <div className="date-fields" aria-label={`${label} date and time`}>
        <label>
          <span className="sr-only">Day</span>
          <input type="number" min="1" max="31" disabled={!enabled} />
        </label>
        <label>
          <span className="sr-only">Month</span>
          <input type="number" min="1" max="12" disabled={!enabled} />
        </label>
        <label>
          <span className="sr-only">Year</span>
          <input className="year" type="number" min="2024" max="2100" disabled={!enabled} />
        </label>
        <label>
          <span className="sr-only">Hour</span>
          <input type="number" min="0" max="23" disabled={!enabled} />
        </label>
        <label>
          <span className="sr-only">Minute</span>
          <input type="number" min="0" max="59" disabled={!enabled} />
        </label>
        <button type="button" className="calendar-button" disabled={!enabled} aria-label={`Open ${label} calendar`}>
          <img src={`${A}/3161f.png`} alt="" />
        </button>
      </div>
    </div>
  );
}

function SectionTitle({ icon, children }: { icon: string; children: React.ReactNode }) {
  return (
    <h2 className="section-title">
      <img src={`${A}/${icon}`} alt="" />
      <span>{children}</span>
    </h2>
  );
}

export default function AssignmentSubmissionSetting() {
  const [activeNav, setActiveNav] = useState('Assignments');
  const [activeTab, setActiveTab] = useState('Assignments');
  const [editMode, setEditMode] = useState(true);
  const [enabledDates, setEnabledDates] = useState([true, true, true]);
  const [onlineText, setOnlineText] = useState(false);
  const [fileSubmission, setFileSubmission] = useState(true);
  const [notice, setNotice] = useState('');

  const showNotice = (message: string) => {
    setNotice(message);
    window.setTimeout(() => setNotice(''), 2500);
  };

  return (
    <div className="app-shell">
      <aside className="sidebar" aria-label="Primary navigation">
        <div className="brand">
          <img src={`${A}/580ec.png`} alt="" className="brand-logo" />
          <div>
            <strong>University Course</strong>
            <span>Management System</span>
          </div>
        </div>

        <nav className="side-nav">
          {navItems.map(([label, icon]) => (
            <button
              type="button"
              key={label}
              className={activeNav === label ? 'active' : ''}
              onClick={() => setActiveNav(label)}
            >
              <img src={`${A}/${icon}`} alt="" />
              <span>{label}</span>
            </button>
          ))}
        </nav>

        <button type="button" className="logout" onClick={() => showNotice('You have been logged out.')}>
          <img src={`${A}/c8ef2.svg`} alt="" />
          <span>LOGOUT</span>
        </button>
      </aside>

      <main className="main">
        <section className="assignment-card" aria-label="Assignment submission settings">
          <div className="card-toolbar">
            <span>Edit Mode</span>
            <button
              type="button"
              className={`mode-switch ${editMode ? 'on' : ''}`}
              onClick={() => setEditMode((value) => !value)}
              aria-label="Toggle edit mode"
              aria-pressed={editMode}
            >
              <img src={`${A}/01f55.png`} alt="" />
            </button>
          </div>

          <p className="breadcrumbs">
            School of Computing / HND in Computing/ Cohorts / HND in Computing - UOK 001 / First Semester Assignments/
            Programming Assignment
          </p>

          <h1>
            <img src={`${A}/11c57.png`} alt="" />
            <span>Unit 01 : Programming Assignment</span>
          </h1>

          <div className="tabs" role="tablist" aria-label="Assignment sections">
            {['Assignments', 'Settings', 'Submissions'].map((tab) => (
              <button
                key={tab}
                type="button"
                role="tab"
                aria-selected={activeTab === tab}
                className={activeTab === tab ? 'active' : ''}
                onClick={() => setActiveTab(tab)}
              >
                {tab}
              </button>
            ))}
          </div>
          <div className="design-rule top-rule">
            <img src={`${A}/393bd.svg`} alt="" />
          </div>

          <form
            onSubmit={(event) => {
              event.preventDefault();
              showNotice('Assignment settings saved.');
            }}
          >
            <fieldset disabled={!editMode}>
              <section className="form-section general">
                <SectionTitle icon="93d4e.png">General</SectionTitle>
                <div className="general-grid">
                  <label htmlFor="assignment-name">Assignment Name :</label>
                  <input id="assignment-name" name="assignmentName" type="text" />
                  <label htmlFor="description">Description / Instructions /<br />Additional Files:</label>
                  <textarea id="description" name="description" />
                </div>
              </section>

              <div className="design-rule section-rule">
                <img src={`${A}/02b60.svg`} alt="" />
              </div>

              <section className="form-section availability">
                <SectionTitle icon="d1018.png">Availability</SectionTitle>
                <div className="date-list">
                  {dates.map((label, index) => (
                    <DateRow
                      key={label}
                      label={label}
                      enabled={enabledDates[index]}
                      onToggle={() =>
                        setEnabledDates((values) => values.map((value, i) => (i === index ? !value : value)))
                      }
                    />
                  ))}
                </div>
              </section>

              <div className="design-rule section-rule lower-rule">
                <img src={`${A}/ac057.svg`} alt="" />
              </div>

              <section className="form-section submission">
                <SectionTitle icon="84a3a.png">Submission Types</SectionTitle>
                <div className="submission-grid">
                  <span>Submission Type :</span>
                  <div className="submission-checks">
                    <label>
                      <Check checked={onlineText} onChange={() => setOnlineText((value) => !value)} label="Online text" />
                      <span>Online text</span>
                    </label>
                    <label>
                      <Check
                        checked={fileSubmission}
                        onChange={() => setFileSubmission((value) => !value)}
                        label="File submission"
                      />
                      <span>File submission</span>
                    </label>
                  </div>
                  <label htmlFor="file-count">Maximum No. of Uploaded Files :</label>
                  <span className="number-wrap">
                    <input id="file-count" type="number" min="0" />
                    <img src={`${A}/8bfd4.png`} alt="" />
                  </span>
                  <label htmlFor="submission-size">Maximum Submission Size :</label>
                  <span className="number-wrap">
                    <input id="submission-size" type="number" min="0" />
                    <img src={`${A}/8bfd4.png`} alt="" />
                  </span>
                </div>
              </section>
            </fieldset>

            <div className="form-actions">
              <button type="submit" className="save-button" disabled={!editMode}>
                Save and Display
              </button>
              <button type="reset" className="cancel-button" onClick={() => showNotice('Changes cancelled.')}>
                Cancle
              </button>
            </div>
          </form>
        </section>
      </main>

      {notice && <div className="toast" role="status">{notice}</div>}
    </div>
  );
}
